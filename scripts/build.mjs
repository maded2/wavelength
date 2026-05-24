#!/usr/bin/env node

/**
 * Cross-platform build script for Wavelength.
 *
 * Usage:
 *   node scripts/build.mjs              — build for current platform
 *   node scripts/build.mjs linux/amd64  — cross-compile for linux/amd64
 *   node scripts/build.mjs --all         — build for all platforms
 */

import { execSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");
const OUT_DIR = join(ROOT, "dist");
const BINARY_NAME = "wavelength";
const CMD = "cmd/server/main.go";

const PLATFORMS = [
  { goos: "linux", goarch: "amd64", suffix: "linux-amd64", ext: "" },
  { goos: "linux", goarch: "arm64", suffix: "linux-arm64", ext: "" },
  { goos: "darwin", goarch: "amd64", suffix: "darwin-amd64", ext: "" },
  { goos: "darwin", goarch: "arm64", suffix: "darwin-arm64", ext: "" },
  { goos: "windows", goarch: "amd64", suffix: "windows-amd64", ext: ".exe" },
  { goos: "windows", goarch: "arm64", suffix: "windows-arm64", ext: ".exe" },
];

// Detect current platform
const currentGoos = execSync("go env GOOS", { cwd: ROOT, encoding: "utf-8" })
  .trim();
const currentGoarch = execSync("go env GOARCH", { cwd: ROOT, encoding: "utf-8" })
  .trim();

function buildForPlatform(platform) {
  const { goos, goarch, suffix, ext } = platform;
  const outputName = `${BINARY_NAME}-${suffix}${ext}`;
  const outputPath = join(OUT_DIR, outputName);

  console.log(`Building ${goos}/${goarch} -> ${outputName} ...`);

  const env = {
    ...process.env,
    GOOS: goos,
    GOARCH: goarch,
  };

  // On Windows, set env differently for execSync
  execSync(`go build -o "${outputPath}" ${CMD}`, {
    cwd: ROOT,
    env,
    stdio: "inherit",
  });

  console.log(`  ✓ ${outputPath}`);
}

function buildCurrent() {
  const isWindows = currentGoos === "windows";
  const outputName = `${BINARY_NAME}${isWindows ? ".exe" : ""}`;

  console.log(`Building for current platform (${currentGoos}/${currentGoarch}) -> ${outputName} ...`);

  execSync(`go build -o "${outputName}" ${CMD}`, {
    cwd: ROOT,
    stdio: "inherit",
  });

  console.log(`  ✓ ${outputName}`);
}

function buildAll() {
  mkdirSync(OUT_DIR, { recursive: true });
  for (const platform of PLATFORMS) {
    buildForPlatform(platform);
  }
  console.log(`\nAll builds complete. Outputs in dist/`);
}

// Parse arguments
const args = process.argv.slice(2);

if (args.length === 0) {
  // Default: build for current platform
  buildCurrent();
} else if (args.includes("--all")) {
  buildAll();
} else {
  // Build for specified platform(s)
  mkdirSync(OUT_DIR, { recursive: true });
  for (const target of args) {
    const platform = PLATFORMS.find(
      (p) => `${p.goos}/${p.goarch}` === target
    );
    if (!platform) {
      console.error(
        `Unknown platform: ${target}\nAvailable: ${PLATFORMS.map((p) => `${p.goos}/${p.goarch}`).join(", ")}`
      );
      process.exit(1);
    }
    buildForPlatform(platform);
  }
}
