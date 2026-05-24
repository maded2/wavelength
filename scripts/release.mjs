#!/usr/bin/env node

/**
 * Release script for Wavelength.
 *
 * Builds all platforms, creates tarballs, and uploads to GitHub Releases.
 *
 * Usage:
 *   GITHUB_TOKEN=ghp_xxx npm run release
 *
 * Requires:
 *   - Go installed
 *   - GITHUB_TOKEN env var with `repo` scope
 *   - tar (Unix) or 7z (Windows) for archiving
 */

import { execSync } from "node:child_process";
import { mkdirSync, rmSync, existsSync, statSync } from "node:fs";
import { join, basename } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");
const OUT_DIR = join(ROOT, "dist");

const PLATFORMS = [
  { goos: "linux", goarch: "amd64", suffix: "linux-amd64", archive: "tar.gz", binary: "wavelength" },
  { goos: "linux", goarch: "arm64", suffix: "linux-arm64", archive: "tar.gz", binary: "wavelength" },
  { goos: "darwin", goarch: "amd64", suffix: "darwin-amd64", archive: "tar.gz", binary: "wavelength" },
  { goos: "darwin", goarch: "arm64", suffix: "darwin-arm64", archive: "tar.gz", binary: "wavelength" },
  { goos: "windows", goarch: "amd64", suffix: "windows-amd64", archive: "zip", binary: "wavelength.exe" },
  { goos: "windows", goarch: "arm64", suffix: "windows-arm64", archive: "zip", binary: "wavelength.exe" },
];

// Read package.json
const pkg = JSON.parse(
  execSync(`node -e "console.log(JSON.stringify(require('./package.json')))"`, {
    cwd: ROOT,
    encoding: "utf-8",
  })
);

const VERSION = pkg.version;
const REPO = pkg.repository?.url || pkg.repository;
const GITHUB_REPO = REPO
  ? REPO.replace(/^git\+/, "").replace(/\.git$/, "").replace(/^https?:\/\//, "").replace(/^github\.com(\:|\/)/, "")
  : null;

if (!GITHUB_REPO) {
  console.error("✗ No repository.url in package.json.");
  process.exit(1);
}

const GITHUB_TOKEN = process.env.GITHUB_TOKEN;
if (!GITHUB_TOKEN) {
  console.error("✗ GITHUB_TOKEN environment variable is required.");
  console.error("  Create a token with `repo` scope at https://github.com/settings/tokens");
  console.error("  Usage: GITHUB_TOKEN=ghp_xxx npm run release");
  process.exit(1);
}

const [OWNER, REPO_NAME] = GITHUB_REPO.split("/");

async function createRelease(tag, name) {
  const url = `https://api.github.com/repos/${OWNER}/${REPO_NAME}/releases`;
  const response = await fetch(url, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${GITHUB_TOKEN}`,
      Accept: "application/vnd.github.v3+json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      tag_name: tag,
      name: name,
      draft: false,
      prerelease: false,
      generate_release_notes: true,
    }),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`Failed to create release: ${response.status} ${text}`);
  }
  return response.json();
}

async function uploadAsset(releaseId, filePath, fileName) {
  const ext = fileName.split(".").slice(-1)[0];
  const contentType = ext === "zip" ? "application/zip" : "application/gzip";
  const url = `https://uploads.github.com/repos/${OWNER}/${REPO_NAME}/releases/${releaseId}/assets?name=${fileName}`;

  const buffer = execSync(`cat "${filePath}"`);

  const response = await fetch(url, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${GITHUB_TOKEN}`,
      Accept: "application/vnd.github.v3+json",
      "Content-Type": contentType,
      "Content-Length": String(buffer.length),
    },
    body: buffer,
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`Failed to upload ${fileName}: ${response.status} ${text}`);
  }
  return response.json();
}

function buildPlatform(platform) {
  const { goos, goarch, suffix, binary } = platform;
  const outputName = binary.startsWith("wavelength-") ? binary : `wavelength-${suffix}${binary.endsWith(".exe") ? "" : ""}`;
  const actualBinary = binary.endsWith(".exe") ? `wavelength-${suffix}.exe` : `wavelength-${suffix}`;
  const outputPath = join(OUT_DIR, actualBinary);

  console.log(`  Building ${goos}/${goarch} -> ${actualBinary} ...`);

  execSync(`go build -o "${outputPath}" cmd/server/main.go`, {
    cwd: ROOT,
    env: { ...process.env, GOOS: goos, GOARCH: goarch },
    stdio: ["inherit", "pipe", "inherit"],
  });

  return outputPath;
}

function createArchive(platform, binaryPath) {
  const { suffix, archive, binary } = platform;
  const archiveName = `wavelength-${VERSION}-${suffix}.${archive}`;
  const archivePath = join(OUT_DIR, archiveName);
  const dirName = `wavelength-${VERSION}-${suffix}`;
  const dirPath = join(OUT_DIR, dirName);

  // Create directory with binary
  mkdirSync(dirPath, { recursive: true });
  execSync(`cp "${binaryPath}" "${join(dirPath, binary)}"`);

  // Create archive
  const isWindows = process.platform === "win32";
  if (archive === "tar.gz") {
    execSync(`tar czf "${archivePath}" -C "${OUT_DIR}" "${dirName}"`, { stdio: "pipe" });
  } else if (archive === "zip") {
    if (isWindows) {
      execSync(`powershell -Command "Compress-Archive -Path '${dirPath}\\*' -DestinationPath '${archivePath}' -Force"`, {
        stdio: "pipe",
      });
    } else {
      execSync(`zip -r "${archivePath}" "${dirName}" -C "${OUT_DIR}"`, { stdio: "pipe" });
    }
  }

  // Cleanup build directory
  rmSync(dirPath, { recursive: true, force: true });

  const size = statSync(archivePath).size;
  console.log(`  ✓ ${archiveName} (${(size / 1024 / 1024).toFixed(1)} MB)`);

  return archivePath;
}

async function run() {
  console.log(`Wavelength v${VERSION} — Release Build`);
  console.log(`Repository: ${OWNER}/${REPO_NAME}`);
  console.log();

  // Clean and create output directory
  if (existsSync(OUT_DIR)) {
    rmSync(OUT_DIR, { recursive: true, force: true });
  }
  mkdirSync(OUT_DIR, { recursive: true });

  // Build all platforms
  console.log("Building all platforms...");
  const archives = [];
  for (const platform of PLATFORMS) {
    const binaryPath = buildPlatform(platform);
    const archivePath = createArchive(platform, binaryPath);
    archives.push({ platform, archivePath });

    // Remove the raw binary (keep only archive)
    rmSync(binaryPath, { force: true });
  }

  console.log();

  // Create GitHub Release
  const tag = `v${VERSION}`;
  console.log(`Creating GitHub Release ${tag} ...`);
  let release;
  try {
    release = await createRelease(tag, `Wavelength v${VERSION}`);
    console.log(`  ✓ Release created: ${release.html_url}`);
  } catch (err) {
    // Check if release already exists
    if (err.message.includes("already exists")) {
      console.log(`  ⚠ Release ${tag} already exists. Uploading assets to existing release.`);
      const existing = await fetch(
        `https://api.github.com/repos/${OWNER}/${REPO_NAME}/releases/tags/${tag}`,
        {
          headers: { Authorization: `Bearer ${GITHUB_TOKEN}`, Accept: "application/vnd.github.v3+json" },
        }
      ).then((r) => r.json());
      release = existing;
    } else {
      throw err;
    }
  }

  // Upload assets
  console.log();
  console.log("Uploading assets...");
  for (const { archivePath } of archives) {
    const fileName = basename(archivePath);
    console.log(`  Uploading ${fileName} ...`);
    try {
      await uploadAsset(release.id, archivePath, fileName);
      console.log(`    ✓ ${fileName}`);
    } catch (err) {
      console.error(`    ✗ ${err.message}`);
    }
  }

  // Cleanup
  rmSync(OUT_DIR, { recursive: true, force: true });

  console.log();
  console.log(`Release complete!`);
  console.log(`  GitHub: ${release.html_url}`);
  console.log(`  Install: npm install github:${OWNER}/${REPO_NAME}`);
}

run().catch((err) => {
  console.error(`\n✗ Release failed: ${err.message}`);
  process.exit(1);
});
