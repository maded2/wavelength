#!/usr/bin/env node

/**
 * Thin wrapper — delegates to the pre-built binary installed by postinstall.
 */

import { execFileSync } from "node:child_process";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");

const isWindows = process.platform === "win32";
const BINARY_NAME = isWindows ? "wavelength.exe" : "wavelength";
const BINARY_PATH = join(ROOT, BINARY_NAME);

try {
  execFileSync(BINARY_PATH, process.argv.slice(2), {
    stdio: "inherit",
  });
} catch {
  // execFileSync throws on non-zero exit; propagate the code
  process.exit(process.exitCode || 1);
}
