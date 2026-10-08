import { expect, test, type Page } from "@playwright/test";
import { agentReadiness } from "../src/renderer/test/agent-readiness-fixtures";
import { installFakeAgent } from "./support/fake-bridge";
import { openInspector } from "./support/open-inspector";

const origin = `http://127.0.0.1:${process.env.AO_E2E_PORT ?? 5173}`;
const projectId = "feedback-visibility";
const sessionId = "feedback-visibility-worker";
const filePath = "src/long-file.ts";
const content = Array.from({ length: 220 }, (_, index) => `export const value${index + 1} = ${index + 1};`).join("\n") + "\n";
const patch = [
	`diff --git a/${filePath} b/${filePath}`,
	"index 1111111..2222222 100644",
	`--- a/${filePath}`,
	`+++ b/${filePath}`,
	"@@ -118,3 +118,4 @@",
	" export const value118 = 118;",
	" export const value119 = 119;",
	" export const value120 = 120;",
	"+export const inserted = true;",
	"",
].join("\n");

function quietEventStream(route: Parameters<Parameters<Page["route"]>[1]>[0]) {
	return route.fulfill({
		status: 200,
		headers: { "content-type": "text/event-stream" },
		body: "retry: 600000\n: connected\n\n",
	});
}

async function openLongFile(page: Page) {
	await installFakeAgent(page, {
		projectId,
		projectName: projectId,
		workers: [{ id: sessionId, provider: "codex", title: "Feedback visibility", mode: "chat" }],
	});

	const file = {
		path: filePath,
		status: "modified",
		additions: 1,
		deletions: 0,
		size: content.length,
		binary: false,
		editable: true,
		fileFingerprint: "fp-1",
	};
	const detail = {
		sessionId,
		...file,
		deleted: false,
		content,
		contentTruncated: false,
		diff: patch,
		diffTruncated: false,
		workspaceVersion: "workspace-1",
	};

	await page.route(`http://127.0.0.1:8080/api/v1/sessions/${sessionId}/**`, async (route) => {
		if (route.request().headers().accept?.includes("text/event-stream")) return quietEventStream(route);
		const url = new URL(route.request().url());
		const endpoint = url.pathname.replace(`/api/v1/sessions/${sessionId}`, "");
		if (endpoint === "/workspace/manifest") {
			return route.fulfill({
				json: {
					sessionId,
					workspaceVersion: "workspace-1",
					files: [file],
					sections: { committed: [], staged: [], unstaged: [file], untracked: [] },
					commits: [],
					summary: { additions: 1, deletions: 0, files: 1 },
					truncated: false,
				},
			});
		}
		if (endpoint === "/workspace/diffs") {
			return route.fulfill({
				json: {
					sessionId,
					workspaceVersion: "workspace-1",
					groups: [{ repository: "", patch, truncated: false, includedPaths: [filePath], deferred: [] }],
				},
			});
		}
		if (endpoint === "/workspace/file") return route.fulfill({ json: detail });
		if (endpoint === "/workspace/history") return route.fulfill({ json: { sessionId, commits: [] } });
		if (endpoint === "/workspace/file/revision") {
			return route.fulfill({
				json: {
					sessionId,
					path: filePath,
					side: "after",
					binary: false,
					content,
					exists: true,
					size: content.length,
					truncated: false,
					workspaceVersion: "workspace-1",
					revision: "after",
				},
			});
		}
		return route.fallback();
	});

	await page.route("http://127.0.0.1:8080/api/v1/**", async (route) => {
		if (route.request().headers().accept?.includes("text/event-stream")) return quietEventStream(route);
		const pathname = new URL(route.request().url()).pathname;
		if (pathname === "/api/v1/agents/readiness" || pathname === "/api/v1/agents/readiness/ensure") {
			return route.fulfill({ json: { agents: [agentReadiness("codex", "Codex")] } });
		}
		if (pathname === `/api/v1/projects/${projectId}`) {
			return route.fulfill({ json: { status: "ok", project: { id: projectId, agent: "codex", config: { worker: { agent: "codex" } } } } });
		}
		if (pathname === `/api/v1/sessions/${sessionId}/conversation`) {
			return route.fulfill({
				json: {
					conversationId: "conversation-feedback-visibility",
					sessionId,
					harness: "codex",
					mode: "chat",
					controller: "ready",
					latestSequence: 0,
					oldestSequence: 0,
					hasMoreBefore: false,
					turns: [],
					messages: [],
					activities: [],
					settings: {},
				},
			});
		}
		if (pathname.endsWith("/conversation/models")) return route.fulfill({ json: { models: [], selected: {} } });
		if (pathname.endsWith("/conversation/skills")) return route.fulfill({ json: { skills: [] } });
		if (pathname.endsWith("/interface-transition")) return route.fulfill({ json: { supported: true, targetMode: "tui" } });
		return route.fulfill({ json: { status: "ok" } });
	});

	await page.goto(`${origin}/#/projects/${projectId}/sessions/${sessionId}`);
	const inspector = await openInspector(page);
	await inspector.getByRole("tab", { name: /Files?$/ }).click();
	await expect(page.getByRole("button", { name: `Collapse ${filePath}` })).toBeVisible();
	await inspector.getByRole("button", { name: "Open full file" }).click();
	await expect(page.getByTestId("session-file-workspace")).toBeVisible();
}

test("@P0 whole-file feedback opens beside the visible sticky toolbar without losing the current code", async ({ page }) => {
	await openLongFile(page);
	const workspace = page.getByTestId("session-file-workspace");
	const scroll = page.getByTestId("session-file-scroll");
	const line120 = workspace.locator('diffs-container [data-line="120"]').first();
	await expect(line120).toBeVisible();
	await line120.evaluate((element) => element.scrollIntoView({ block: "center" }));
	await expect.poll(() => scroll.evaluate((element) => element.scrollTop)).toBeGreaterThan(500);

	const before = await scroll.evaluate((element) => element.scrollTop);
	await workspace.getByRole("button", { name: "Add feedback" }).click();

	const composer = workspace.getByPlaceholder("Describe what the agent should change...");
	await expect(composer).toBeVisible();
	const [composerBox, scrollBox] = await Promise.all([composer.boundingBox(), scroll.boundingBox()]);
	expect(composerBox).not.toBeNull();
	expect(scrollBox).not.toBeNull();
	expect(composerBox!.y).toBeGreaterThanOrEqual(scrollBox!.y);
	expect(composerBox!.y + composerBox!.height).toBeLessThanOrEqual(scrollBox!.y + scrollBox!.height);
	await expect(line120).toBeVisible();

	const after = await scroll.evaluate((element) => element.scrollTop);
	expect(Math.abs(after - before)).toBeLessThan(8);
});
