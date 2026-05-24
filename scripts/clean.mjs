#!/usr/bin/env node

/**
 * Cross-platform clean script for Wavelength.
 *
 * Removes build artifacts (binary and dist/ directory).
 *
 * Usage:
 *   npm run clean
 */

import { execSync } from "node:child_process";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");

// Detect current platform
const currentGoos = execSync("go env GOOS", { cwd: ROOT, encoding: "utf-8" })
  .trim();

const isWindows = currentGoos === "windows";
const BINARY_NAME = isWindows ? "wavelength.exe" : "wavelength";
const BINARY_PATH = join(ROOT, BINARY_NAME);

// Use Node.js fs for cross-platform removal
import { rmSync, existsSync } from "node:fs";

const distDir = join(ROOT, "dist");

// Remove current binary
if (existsSync(BINARY_PATH)) {
  rmSync(BINARY_PATH);
  console.log(`Removed ${BINARY_NAME}`);
}

// Remove dist directory
if (existsSync(distDir)) {
  rmSync(distDir, { recursive: true });
  console.log("Removed dist/");
}

console.log("Clean complete.");
