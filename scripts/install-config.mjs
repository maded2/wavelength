#!/usr/bin/env node

/**
 * Install the default config file to the standard location for the current platform.
 *
 * Usage:
 *   npm run install-config
 *
 * Standard locations:
 *   Linux:   ~/.config/wavelength/config.json
 *   macOS:   ~/.config/wavelength/config.json
 *   Windows: %APPDATA%/wavelength/config.json
 *
 * If the file already exists, it is preserved (not overwritten).
 */

import { copyFileSync, mkdirSync, existsSync, readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const ROOT = join(__dirname, "..");
const SOURCE_CONFIG = join(ROOT, "configs/config.json");

function getConfigDir() {
  if (process.platform === "win32") {
    const appData = process.env.APPDATA || join(homedir(), "AppData", "Roaming");
    return join(appData, "wavelength");
  } else {
    // Linux and macOS: XDG Base Directory
    return join(homedir(), ".config", "wavelength");
  }
}

function getConfigPath() {
  return join(getConfigDir(), "config.json");
}

function run() {
  const destDir = getConfigDir();
  const destPath = getConfigPath();

  // Check source exists
  if (!existsSync(SOURCE_CONFIG)) {
    console.error(`✗ Source config not found: ${SOURCE_CONFIG}`);
    process.exit(1);
  }

  // Check if config already exists at destination
  if (existsSync(destPath)) {
    console.log(`⚠ Config already exists at: ${destPath}`);
    console.log("  Skipping — existing file preserved.");
    return;
  }

  // Create directory
  mkdirSync(destDir, { recursive: true });

  // Copy config
  copyFileSync(SOURCE_CONFIG, destPath);

  console.log(`✓ Installed config to: ${destPath}`);
  console.log();
  console.log("  Edit the file with your LLM endpoint, model, and API key.");
  console.log();
  console.log("  Start the server:");
  console.log(`    ./wavelength -config ${destPath}`);
}

run();
