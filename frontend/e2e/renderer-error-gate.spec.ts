import { expect, test } from "./support/test";

test("renderer error gate permits a clean page", async ({ page }) => {
	await page.setContent("<main>clean renderer</main>");
	await expect(page.getByText("clean renderer")).toBeVisible();
});

test.fail("renderer error gate rejects an uncaught page error", async ({ page }) => {
	await page.setContent("<main>synthetic pageerror</main>");
	await page.evaluate(() => {
		setTimeout(() => {
			throw new Error("AO synthetic pageerror gate");
		}, 0);
	});
	await page.waitForTimeout(50);
});

test("renderer error gate does not confuse browser network diagnostics with console.error", async ({ page }) => {
	await page.route("**/synthetic-network-failure.png", async (route) => {
		await route.fulfill({ status: 502, contentType: "text/plain", body: "synthetic bad gateway" });
	});
	await page.setContent('<main>network diagnostic control<img src="/synthetic-network-failure.png" /></main>');
	await page.waitForTimeout(50);
	await expect(page.getByText("network diagnostic control")).toBeVisible();
});

test.fail("renderer error gate rejects error-level console output", async ({ page }) => {
	await page.setContent("<main>synthetic console error</main>");
	await page.evaluate(() => {
		console.error("AO synthetic console-error gate");
	});
	await page.waitForTimeout(10);
});


test.describe("documented exact renderer error allowance", () => {
	test.use({
		rendererErrorAllowlist: [
			{
				kind: "console",
				exact: "AO synthetic documented console error",
				issue: "#3787",
			},
		],
	});

	test("allows only the exact documented signature", async ({ page }) => {
		await page.setContent("<main>documented allowance</main>");
		await page.evaluate(() => {
			console.error("AO synthetic documented console error");
		});
		await expect(page.getByText("documented allowance")).toBeVisible();
	});
});
