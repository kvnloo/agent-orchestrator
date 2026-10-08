import {
	test as base,
	expect,
	_electron,
	type ConsoleMessage,
	type TestInfo,
} from "@playwright/test";

export type RendererErrorKind = "console" | "pageerror";

export type RendererErrorAllowance = {
	/** Exact browser message to allow. Substrings/regexes are intentionally unsupported. */
	exact: string;
	/** Required issue reference documenting why the exception is temporarily safe. */
	issue: `#${number}` | `https://github.com/OrchestratorInc/agent-orchestrator/issues/${number}`;
	kind: RendererErrorKind;
};

type RendererErrorEvent = {
	kind: RendererErrorKind;
	message: string;
	stack?: string;
	location?: ReturnType<ConsoleMessage["location"]>;
};

type RendererBrowserDiagnostic = {
	kind: "network";
	message: string;
	location?: ReturnType<ConsoleMessage["location"]>;
};

type RendererErrorFixtures = {
	rendererErrorAllowlist: RendererErrorAllowance[];
	_rendererErrorGate: void;
};

function validateAllowance(allowance: RendererErrorAllowance): void {
	if (!allowance.exact) throw new Error("renderer error allowance requires an exact message");
	if (
		!/^#\d+$/.test(allowance.issue) &&
		!/^https:\/\/github\.com\/OrchestratorInc\/agent-orchestrator\/issues\/\d+$/.test(allowance.issue)
	) {
		throw new Error(`renderer error allowance must link an AO issue: ${allowance.issue}`);
	}
}

function allowed(event: RendererErrorEvent, allowances: RendererErrorAllowance[]): boolean {
	return allowances.some((allowance) => allowance.kind === event.kind && allowance.exact === event.message);
}

function isBrowserNetworkDiagnostic(message: ConsoleMessage): boolean {
	if (message.type() !== "error") return false;
	const text = message.text();
	return (
		text.startsWith("Failed to load resource:") ||
		(text.startsWith("WebSocket connection to '") && text.includes("' failed:"))
	);
}

function gateErrorMessage(events: RendererErrorEvent[]): string {
	const lines = events.map((event) => `- ${event.kind}: ${event.message}`);
	return [
		"Unexpected renderer errors detected by the AO Playwright gate.",
		...lines,
		"See the attached renderer-error-gate.json evidence.",
	].join("\n");
}

async function attachGateEvidence(
	testInfo: TestInfo,
	all: RendererErrorEvent[],
	unexpected: RendererErrorEvent[],
	allowances: RendererErrorAllowance[],
	browserDiagnostics: RendererBrowserDiagnostic[],
): Promise<void> {
	await testInfo.attach("renderer-error-gate", {
		body: Buffer.from(
			`${JSON.stringify({ all, unexpected, allowances, browserDiagnostics }, null, 2)}\n`,
		),
		contentType: "application/json",
	});
}

/**
 * Shared renderer E2E gate.
 *
 * This is an auto fixture rather than a page override so specs that provide
 * their own page fixture (including the native-Electron path) still inherit
 * the gate. Unexpected uncaught page errors and error-level console messages
 * fail the owning test. Exceptions are exact-message-only and must cite an AO
 * issue; broad ResizeObserver/xterm suppression is deliberately impossible.
 */
export const test = base.extend<RendererErrorFixtures>({
	rendererErrorAllowlist: [[], { option: true }],
	_rendererErrorGate: [
		async ({ page, rendererErrorAllowlist }, use, testInfo) => {
			for (const allowance of rendererErrorAllowlist) validateAllowance(allowance);

			const events: RendererErrorEvent[] = [];
			const browserDiagnostics: RendererBrowserDiagnostic[] = [];
			const onPageError = (error: Error) => {
				events.push({
					kind: "pageerror",
					message: error.message,
					stack: error.stack,
				});
			};
			const onConsole = (message: ConsoleMessage) => {
				if (message.type() !== "error") return;
				if (isBrowserNetworkDiagnostic(message)) {
					browserDiagnostics.push({
						kind: "network",
						message: message.text(),
						location: message.location(),
					});
					return;
				}
				events.push({
					kind: "console",
					message: message.text(),
					location: message.location(),
				});
			};

			page.on("pageerror", onPageError);
			page.on("console", onConsole);
			try {
				await use();
			} finally {
				page.off("pageerror", onPageError);
				page.off("console", onConsole);
			}

			const unexpected = events.filter((event) => !allowed(event, rendererErrorAllowlist));
			if (unexpected.length === 0) return;

			await attachGateEvidence(testInfo, events, unexpected, rendererErrorAllowlist, browserDiagnostics);

			// Preserve the original assertion/fixture failure when one already exists.
			// The renderer evidence remains attached, but the gate does not replace the
			// primary failure with a teardown error.
			if (testInfo.errors.length > 0) return;

			throw new Error(gateErrorMessage(unexpected));
		},
		{ auto: true },
	],
});

export { expect, _electron };
export type { Locator, Page } from "@playwright/test";
