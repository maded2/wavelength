#!/usr/bin/env node

/**
 * Cross-platform run script for Wavelength.
 *
 * Builds for the current platform and runs the binary with a config file.
 *
 * Usage:
 *   npm run run                           — build and run with default config
 *   npm run run -- --config my.json       — build and run with custom config
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

// Build first
console.log("Building...");
execSync(`go build -o "${BINARY_PATH}" cmd/server/main.go`, {
  cwd: ROOT,
  stdio: "inherit",
});

// Parse extra args after --
const extraArgs = process.argv.slice(2);
let configArg = "-config configs/config.json";
for (let i = 0; i < extraArgs.length; i++) {
  if (extraArgs[i] === "--config" && extraArgs[i + 1]) {
    configArg = `-config ${extraArgs[i + 1]}`;
    break;
  }
}

console.log(`Running ${BINARY_NAME} ${configArg}`);
execSync(`${BINARY_PATH} ${configArg}`, {
  cwd: ROOT,
  stdio: "inherit",
});
