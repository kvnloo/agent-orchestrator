#!/usr/bin/env node

import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

const scriptsDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptsDir, "..");
const stampRoot = join(repoRoot, ".ao", "bootstrap");
const supportedNodeMajor = 24;
const supportedNpmMajor = 11;

export function packageRoots() {
	return [".", "packages/product-ui", "packages/cloud-client", "frontend", "frontend/src/docs"];
}

export function nativeVerificationModules() {
	return ["vite", "better-sqlite3"];
}

function major(version) {
	const match = String(version ?? "").trim().replace(/^v/, "").match(/^(\d+)/);
	return match ? Number(match[1]) : Number.NaN;
}

export function assertSupportedRuntime({ node, npm }) {
	const nodeMajor = major(node);
	const npmMajor = major(npm);
	if (nodeMajor === supportedNodeMajor && npmMajor === supportedNpmMajor) return;
	throw new Error(
		[
			`AO frontend bootstrap requires Node 24 and npm 11; found Node ${node || "unknown"} and npm ${npm || "unknown"}.`,
			"Use the repository .node-version or .nvmrc before installing dependencies.",
			"Examples: `nvm use`, `mise install`, or `nodenv install`.",
		].join("\n"),
	);
}

export function bootstrapCacheKey({ lockfileHash, node, npm, platform, arch }) {
	return createHash("sha256")
		.update(JSON.stringify({ lockfileHash, node, npm, platform, arch }))
		.digest("hex");
}

function npmExecutable(platform = process.platform) {
	return platform === "win32" ? "npm.cmd" : "npm";
}

function run(command, args, options = {}) {
	const result = spawnSync(command, args, {
		cwd: options.cwd ?? repoRoot,
		encoding: "utf8",
		stdio: options.capture ? "pipe" : "inherit",
		env: process.env,
	});
	if (result.error) throw result.error;
	if (result.status !== 0) {
		const detail = options.capture ? `${result.stdout ?? ""}${result.stderr ?? ""}`.trim() : "";
		throw new Error(
			`${command} ${args.join(" ")} failed with exit code ${result.status}${detail ? `\n${detail}` : ""}`,
		);
	}
	return String(result.stdout ?? "").trim();
}

function lockfileHash(root) {
	const lockfile = join(repoRoot, root, "package-lock.json");
	if (!existsSync(lockfile)) throw new Error(`Missing lockfile: ${lockfile}`);
	return createHash("sha256").update(readFileSync(lockfile)).digest("hex");
}

function stampPath(root) {
	const slug = root === "." ? "root" : root.replaceAll(/[\\/]/g, "__");
	return join(stampRoot, `${slug}.json`);
}

function readStamp(root) {
	try {
		return JSON.parse(readFileSync(stampPath(root), "utf8"));
	} catch {
		return null;
	}
}

function writeStamp(root, key) {
	mkdirSync(stampRoot, { recursive: true });
	writeFileSync(stampPath(root), `${JSON.stringify({ key, completedAt: new Date().toISOString() }, null, 2)}\n`);
}

function nativeProbeScript(moduleName) {
	switch (moduleName) {
		case "vite":
			return 'import("vite").then(() => undefined)';
		case "better-sqlite3":
			return 'const Database = require("better-sqlite3"); const db = new Database(":memory:"); db.close();';
		default:
			throw new Error(`Unknown native verification module: ${moduleName}`);
	}
}

function verifyFrontendBindings({ capture = false } = {}) {
	for (const moduleName of nativeVerificationModules()) {
		run(process.execPath, ["--eval", nativeProbeScript(moduleName)], {
			cwd: join(repoRoot, "frontend"),
			capture,
		});
	}
}

function currentNpmVersion() {
	return run(npmExecutable(), ["--version"], { capture: true });
}

function packageKey(root, npmVersion) {
	return bootstrapCacheKey({
		lockfileHash: lockfileHash(root),
		node: process.versions.node,
		npm: npmVersion,
		platform: process.platform,
		arch: process.arch,
	});
}

function installPackageRoot(root, npmVersion, { force = false } = {}) {
	const cwd = join(repoRoot, root);
	const key = packageKey(root, npmVersion);
	const installed = existsSync(join(cwd, "node_modules"));
	const cached = readStamp(root)?.key === key;
	let verified = true;
	if (root === "frontend" && installed && cached) {
		try {
			verifyFrontendBindings({ capture: true });
		} catch {
			verified = false;
		}
	}
	if (!force && installed && cached && verified) {
		console.log(`bootstrap: reuse ${root}`);
		return;
	}

	console.log(`bootstrap: install ${root}`);
	run(npmExecutable(), ["ci", "--include=optional", "--no-audit", "--no-fund"], { cwd });
	if (root === "frontend") {
		try {
			verifyFrontendBindings({ capture: true });
		} catch (firstError) {
			console.warn("bootstrap: frontend native verification failed; retrying one clean install");
			rmSync(join(cwd, "node_modules"), { force: true, recursive: true });
			run(npmExecutable(), ["ci", "--include=optional", "--no-audit", "--no-fund"], { cwd });
			try {
				verifyFrontendBindings({ capture: true });
			} catch (secondError) {
				throw new Error(
					[
						"Frontend dependencies installed, but native verification still failed after one clean retry.",
						`First failure: ${firstError.message}`,
						`Second failure: ${secondError.message}`,
						"Remove frontend/node_modules and rerun `npm run bootstrap:frontend`, then include the full output in the issue.",
					].join("\n"),
				);
			}
		}
	}
	writeStamp(root, key);
}

export function validateBootstrapInputs(npmVersion = currentNpmVersion()) {
	assertSupportedRuntime({ node: process.versions.node, npm: npmVersion });
	for (const root of packageRoots()) lockfileHash(root);
	return npmVersion;
}

export function bootstrap({ check = false, force = false } = {}) {
	const npmVersion = validateBootstrapInputs();
	if (check) {
		console.log(`bootstrap: runtime OK (Node ${process.versions.node}, npm ${npmVersion})`);
		return;
	}
	for (const root of packageRoots()) installPackageRoot(root, npmVersion, { force });
	verifyFrontendBindings();
	console.log("bootstrap: frontend worktree ready");
}

const invokedPath = process.argv[1] ? pathToFileURL(resolve(process.argv[1])).href : "";
if (import.meta.url === invokedPath) {
	try {
		bootstrap({
			check: process.argv.includes("--check"),
			force: process.argv.includes("--force"),
		});
	} catch (error) {
		console.error(error instanceof Error ? error.message : error);
		process.exitCode = 1;
	}
}
