#!/usr/bin/env node

/**
 * Post-install for Wavelength.
 *
 * Downloads the pre-built binary for the current platform from GitHub Releases.
 * Falls back to building from source if the download fails.
 *
 * Skips if SKIP_BUILD env var is set (useful for CI or packaging).
 */

import { execSync } from "node:child_process";
import { mkdirSync, writeFileSync, chmodSync, rmSync, existsSync, readFileSync } from "node:fs";
import { join, basename } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");

// Allow skipping (e.g., in CI, Docker, or when packaging)
if (process.env.SKIP_BUILD) {
  console.log("⏭ SKIP_BUILD is set — skipping post-install build.");
  process.exit(0);
}

// Read package.json for version and repo info
const pkg = JSON.parse(readFileSync(join(ROOT, "package.json"), "utf-8"));

const VERSION = pkg.version;
const REPO = pkg.repository?.url || pkg.repository;
const GITHUB_REPO = REPO
  ? REPO.replace(/^git\+/, "").replace(/\.git$/, "").replace(/^https?:\/\//, "").replace(/^github\.com(\:|\/)/, "")
  : null;

if (!GITHUB_REPO) {
  console.error("⚠ No repository.url in package.json — cannot download from GitHub Releases.");
  console.error("  Falling back to local build.");
  buildFromSource();
  process.exit(0);
}

// Map npm platform/arch to GitHub asset names
const PLATFORM_MAP = {
  "linux/x64":     { asset: `wavelength-${VERSION}-linux-amd64.tar.gz`, binary: "wavelength" },
  "linux/arm64":   { asset: `wavelength-${VERSION}-linux-arm64.tar.gz`, binary: "wavelength" },
  "darwin/x64":    { asset: `wavelength-${VERSION}-darwin-amd64.tar.gz`, binary: "wavelength" },
  "darwin/arm64":  { asset: `wavelength-${VERSION}-darwin-arm64.tar.gz`, binary: "wavelength" },
  "win32/x64":     { asset: `wavelength-${VERSION}-windows-amd64.zip`, binary: "wavelength.exe" },
  "win32/arm64":   { asset: `wavelength-${VERSION}-windows-arm64.zip`, binary: "wavelength.exe" },
};

const platformKey = `${process.platform}/${process.arch}`;
const platform = PLATFORM_MAP[platformKey];

if (!platform) {
  console.error(`⚠ Unsupported platform: ${platformKey}`);
  console.error("  Falling back to local build (requires Go).");
  buildFromSource();
  process.exit(0);
}

const { asset, binary } = platform;
const BINARY_PATH = join(ROOT, binary);
const TMP_DIR = join(ROOT, ".tmp-install");
const ASSET_PATH = join(TMP_DIR, asset);

const DOWNLOAD_URL = `https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${asset}`;

async function downloadFromGitHub() {
  console.log(`Post-install: downloading ${asset} from GitHub Releases ...`);
  console.log(`  ${DOWNLOAD_URL}`);

  // Create temp directory
  mkdirSync(TMP_DIR, { recursive: true });

  // Download the asset
  const response = await fetch(DOWNLOAD_URL);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
  }

  const buffer = Buffer.from(await response.arrayBuffer());
  writeFileSync(ASSET_PATH, buffer);
  console.log(`  ✓ Downloaded ${asset} (${(buffer.length / 1024 / 1024).toFixed(1)} MB)`);

  // Extract using system tools
  const ext = asset.endsWith(".zip") ? "zip" : "tar.gz";

  if (ext === "tar.gz") {
    execSync(`tar xzf "${ASSET_PATH}" -C "${TMP_DIR}"`, { stdio: "pipe" });
  } else if (ext === "zip") {
    const isWindows = process.platform === "win32";
    if (isWindows) {
      execSync(
        `powershell -Command "Expand-Archive -Path '${ASSET_PATH}' -DestinationPath '${TMP_DIR}' -Force"`,
        { stdio: "pipe" }
      );
    } else {
      execSync(`unzip -o "${ASSET_PATH}" -d "${TMP_DIR}"`, { stdio: "pipe" });
    }
  }

  // Find the extracted binary
  let extractedBinary = join(TMP_DIR, binary);
  if (!existsSync(extractedBinary)) {
    // Try in a subdirectory (e.g., wavelength-0.1.0-linux-amd64/wavelength)
    const dirName = asset.replace(/\.tar\.gz$/, "").replace(/\.zip$/, "");
    extractedBinary = join(TMP_DIR, dirName, binary);
  }

  if (!existsSync(extractedBinary)) {
    throw new Error(`Binary not found in archive: ${binary}`);
  }

  // Copy binary to ROOT
  const content = readFileSync(extractedBinary);
  writeFileSync(BINARY_PATH, content);

  // Make executable (not on Windows)
  if (process.platform !== "win32") {
    chmodSync(BINARY_PATH, 0o755);
  }

  // Cleanup
  rmSync(TMP_DIR, { recursive: true, force: true });

  console.log(`✓ Installed ${binary} from GitHub Releases.`);
}

function buildFromSource() {
  try {
    execSync("go version", { encoding: "utf-8", stdio: "pipe" });
  } catch {
    console.error("✗ Go is not installed and no pre-built binary could be downloaded.");
    console.error("  Install Go 1.22+ from https://go.dev/dl/");
    return;
  }

  console.log("Post-install: building from source ...");
  try {
    execSync(`go build -o "${BINARY_PATH}" cmd/server/main.go`, {
      cwd: ROOT,
      stdio: ["inherit", "pipe", "inherit"],
    });
    console.log(`✓ Built ${binary} from source.`);
  } catch {
    console.error(`✗ Failed to build ${binary} from source.`);
    console.error("  Try installing Go or check your network connection for GitHub Releases.");
  }
}

// Run
downloadFromGitHub().catch((err) => {
  console.error(`⚠ Could not download from GitHub Releases: ${err.message}`);
  console.error("  Falling back to building from source (requires Go).");
  // Cleanup temp dir on failure
  if (existsSync(TMP_DIR)) {
    rmSync(TMP_DIR, { recursive: true, force: true });
  }
  buildFromSource();
});
