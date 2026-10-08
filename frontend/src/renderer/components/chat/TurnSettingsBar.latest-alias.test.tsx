import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import type { ChatConfigOption } from "../../types/conversation";
import { TurnSettingsBar } from "./TurnSettingsBar";

function claudeModelOption(currentValue: string): ChatConfigOption {
	return {
		id: "model",
		name: "Model",
		category: "model",
		type: "select",
		currentValue,
		choices: [
			{ value: "default", name: "Default (recommended)", description: "Opus" },
			{
				value: "opus",
				name: "Opus",
				description: "Opus 5.5 · Best for everyday, complex tasks",
			},
			{
				value: "claude-opus-5",
				name: "Opus 5",
				description: "Newer version available · select Opus for Opus 5.5",
			},
		],
	};
}

function renderPicker(currentValue = "default", option = claudeModelOption(currentValue)) {
	const user = userEvent.setup();
	const onChange = vi.fn();
	render(
		<TurnSettingsBar
			harness="claude-code"
			models={[]}
			settings={{}}
			configOptions={[option]}
			onChangeConfigOption={onChange}
		/>,
	);
	return { user, onChange, picker: screen.getByRole("button", { name: "Model" }) };
}

it("shows the concrete latest Claude version without hiding its alias behind Default", async () => {
	const { user, picker } = renderPicker();
	expect(picker).toHaveTextContent("Use agent model (Opus 5.5)");

	await user.click(picker);
	const follow = screen.getByRole("menuitemradio", { name: "Use agent model (Opus 5.5)" });
	const explicit = screen.getByRole("menuitemradio", { name: "Opus 5.5" });
	expect(follow).toHaveAttribute("aria-checked", "true");
	expect(explicit).toHaveAttribute("aria-checked", "false");
	expect(screen.getByRole("menuitemradio", { name: "Opus 5" })).toBeInTheDocument();
	expect(screen.queryByRole("menuitemradio", { name: "Opus", exact: true })).not.toBeInTheDocument();
});

it("labels an explicitly selected latest alias with its concrete version", async () => {
	const { user, picker } = renderPicker("opus");
	expect(picker).toHaveTextContent("Opus 5.5");

	await user.click(picker);
	expect(screen.getByRole("menuitemradio", { name: "Opus 5.5" })).toHaveAttribute(
		"aria-checked",
		"true",
	);
	expect(screen.queryByRole("menuitemradio", { name: /Use agent model/ })).not.toBeInTheDocument();
});

it("still collapses an alias whose description does not expose a concrete version", async () => {
	const option: ChatConfigOption = {
		id: "model",
		name: "Model",
		category: "model",
		type: "select",
		currentValue: "default",
		choices: [
			{ value: "default", name: "Default (recommended)", description: "Sonnet" },
			{ value: "sonnet", name: "Sonnet", description: "Sonnet" },
		],
	};
	const { user, picker } = renderPicker("default", option);
	expect(picker).toHaveTextContent("Sonnet");
	await user.click(picker);
	expect(screen.getAllByRole("menuitemradio", { name: "Sonnet" })).toHaveLength(1);
});
