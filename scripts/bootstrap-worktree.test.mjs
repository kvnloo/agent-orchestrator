import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import test from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

const scriptsDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptsDir, "..");
const bootstrapPath = resolve(scriptsDir, "bootstrap-worktree.mjs");

async function bootstrapModule() {
	assert.equal(existsSync(bootstrapPath), true, "scripts/bootstrap-worktree.mjs must exist");
	return import(pathToFileURL(bootstrapPath).href);
}

test("repository metadata selects the supported frontend runtime", () => {
	assert.equal(readFileSync(resolve(repoRoot, ".node-version"), "utf8").trim(), "24");
	assert.equal(readFileSync(resolve(repoRoot, ".nvmrc"), "utf8").trim(), "24");
	const pkg = JSON.parse(readFileSync(resolve(repoRoot, "package.json"), "utf8"));
	assert.equal(pkg.engines.node, ">=24 <25");
	assert.equal(pkg.engines.npm, ">=11 <12");
	assert.equal(pkg.scripts["bootstrap:frontend"], "node ./scripts/bootstrap-worktree.mjs");
});

test("runtime validation fails early with an actionable selector message", async () => {
	const { assertSupportedRuntime } = await bootstrapModule();
	assert.throws(
		() => assertSupportedRuntime({ node: "18.20.4", npm: "10.8.2" }),
		(error) => {
			assert.match(error.message, /Node 24/);
			assert.match(error.message, /\.node-version/);
			assert.match(error.message, /npm 11/);
			return true;
		},
	);
	assert.doesNotThrow(() => assertSupportedRuntime({ node: "24.9.0", npm: "11.6.2" }));
});

test("bootstrap installs every desktop frontend package root in dependency order", async () => {
	const { packageRoots } = await bootstrapModule();
	assert.deepEqual(packageRoots(), [
		".",
		"packages/product-ui",
		"packages/cloud-client",
		"frontend",
		"frontend/src/docs",
	]);
});

test("cache identity changes for every unsafe reuse boundary", async () => {
	const { bootstrapCacheKey } = await bootstrapModule();
	const base = {
		lockfileHash: "abc",
		node: "24.9.0",
		npm: "11.6.2",
		platform: "darwin",
		arch: "arm64",
	};
	const key = bootstrapCacheKey(base);
	for (const [field, value] of [
		["lockfileHash", "def"],
		["node", "24.10.0"],
		["npm", "11.7.0"],
		["platform", "linux"],
		["arch", "x64"],
	]) {
		assert.notEqual(bootstrapCacheKey({ ...base, [field]: value }), key, `${field} must invalidate bootstrap reuse`);
	}
});

test("native verification covers the build tool and Electron database binding", async () => {
	const { nativeVerificationModules } = await bootstrapModule();
	assert.deepEqual(nativeVerificationModules(), ["vite", "better-sqlite3"]);
});
