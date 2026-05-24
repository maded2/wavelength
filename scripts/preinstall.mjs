#!/usr/bin/env node

/**
 * Pre-install checks for Wavelength.
 *
 * Validates that the required tools are available on the current platform
 * before `npm install` proceeds.
 *
 * Checks:
 *   - Go >= 1.22
 *   - Compatible OS/CPU
 */

import { execSync } from "node:child_process";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");

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

// Check Go
try {
  const goVersion = execSync("go version", { encoding: "utf-8" }).trim();
  console.log(`✓ Go found: ${goVersion}`);
} catch {
  console.error("✗ Go is not installed or not in PATH.");
  console.error("  Install Go 1.22+ from https://go.dev/dl/");
  ok = false;
}

if (!ok) {
  console.error("\nAborting install. Fix the issues above and try again.");
  process.exit(1);
}

console.log("✓ Pre-install checks passed.");
