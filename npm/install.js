#!/usr/bin/env node

const { execFileSync } = require("child_process");
const { createWriteStream, chmodSync, mkdirSync, existsSync, unlinkSync } = require("fs");
const { get } = require("https");
const { join } = require("path");
const { renameSync } = require("fs");

const VERSION = require("./package.json").version;
const REPO = "4ier/notion-cli";

function getPlatform() {
  const platform = process.platform;
  const arch = process.arch;

  const osMap = { linux: "linux", darwin: "darwin", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };

  const os = osMap[platform];
  const cpu = archMap[arch];

  if (!os || !cpu) {
    console.error(`Unsupported platform: ${platform}/${arch}`);
    process.exit(1);
  }

  return { os, cpu };
}

function getDownloadURL(os, cpu) {
  const ext = os === "windows" ? "zip" : "tar.gz";
  return `https://github.com/${REPO}/releases/download/v${VERSION}/notion-cli_${VERSION}_${os}_${cpu}.${ext}`;
}

function download(url) {
  return new Promise((resolve, reject) => {
    get(url, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        return download(res.headers.location).then(resolve).catch(reject);
      }
      if (res.statusCode !== 200) {
        reject(new Error(`Download failed: ${res.statusCode} ${url}`));
        return;
      }
      resolve(res);
    }).on("error", reject);
  });
}

async function install() {
  const { os, cpu } = getPlatform();
  const url = getDownloadURL(os, cpu);
  const binDir = join(__dirname, "bin");
  const srcName = os === "windows" ? "notion.exe" : "notion";
  const destName = os === "windows" ? "notion-bin.exe" : "notion-bin";
  const destPath = join(binDir, destName);

  if (existsSync(destPath)) {
    console.log("notion-cli already installed");
    return;
  }

  console.log(`Downloading notion-cli v${VERSION} for ${os}/${cpu}...`);

  mkdirSync(binDir, { recursive: true });

  const stream = await download(url);

  if (os === "windows") {
    const tmpPath = join(binDir, "tmp.zip");
    const file = createWriteStream(tmpPath);
    await new Promise((resolve, reject) => {
      stream.pipe(file);
      file.on("finish", resolve);
      file.on("error", reject);
    });
    // Extract to a temp dir first to avoid overwriting bin/notion.cmd wrapper
    const tmpDir = join(binDir, "_tmp_extract");
    mkdirSync(tmpDir, { recursive: true });
    try {
      execFileSync("unzip", ["-o", tmpPath, srcName, "-d", tmpDir]);
    } catch {
      execFileSync("powershell", [
        "-Command",
        `Expand-Archive -Path '${tmpPath}' -DestinationPath '${tmpDir}' -Force`,
      ]);
    }
    unlinkSync(tmpPath);
    const extractedPath = join(tmpDir, srcName);
    if (existsSync(extractedPath)) {
      renameSync(extractedPath, destPath);
    }
    try { require("fs").rmdirSync(tmpDir, { recursive: true }); } catch {}
  } else {
    const tmpPath = join(binDir, "tmp.tar.gz");
    const file = createWriteStream(tmpPath);
    await new Promise((resolve, reject) => {
      stream.pipe(file);
      file.on("finish", resolve);
      file.on("error", reject);
    });
    // Extract to a temp dir first to avoid overwriting the shell wrapper (bin/notion)
    const tmpDir = join(binDir, "_tmp_extract");
    mkdirSync(tmpDir, { recursive: true });
    execFileSync("tar", ["xzf", tmpPath, "-C", tmpDir, srcName]);
    unlinkSync(tmpPath);
    const extractedPath = join(tmpDir, srcName);
    if (existsSync(extractedPath)) {
      renameSync(extractedPath, destPath);
    }
    try { require("fs").rmdirSync(tmpDir); } catch {}
  }

  if (os !== "windows") {
    chmodSync(destPath, 0o755);
  }

  console.log(`notion-cli v${VERSION} installed successfully`);
}

install().catch((err) => {
  console.error("Failed to install notion-cli:", err.message);
  console.error("You can install manually from https://github.com/4ier/notion-cli/releases");
  process.exit(1);
});
