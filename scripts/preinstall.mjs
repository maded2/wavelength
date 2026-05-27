#!/usr/bin/env node

/**
 * Pre-install checks for Wavelength.
 *
 * Validates that the current platform is supported
 * before `npm install` proceeds.
 *
 * Checks:
 *   - Compatible OS/CPU
 *
 * Go is NOT required — the binary is downloaded from GitHub Releases.
 */

const SUPPORTED_OS = new Set(["linux", "darwin", "win32"]);
const SUPPORTED_CPU = new Set(["x64", "arm64"]);

let ok = true;

// Check OS
if (!SUPPORTED_OS.has(process.platform)) {
  console.error(
    `⚠ Unsupported OS: ${process.platform}. Supported: ${[...SUPPORTED_OS].join(", ")}`
  );
  ok = false;
}

// Check CPU
if (!SUPPORTED_CPU.has(process.arch)) {
  console.error(
    `⚠ Unsupported CPU: ${process.arch}. Supported: ${[...SUPPORTED_CPU].join(", ")}`
  );
  ok = false;
}

if (!ok) {
  console.error("\nAborting install. Fix the issues above and try again.");
  process.exit(1);
}

console.log("✓ Pre-install checks passed.");
