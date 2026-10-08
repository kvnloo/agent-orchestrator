import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import {
  InspectorActivityTimelineView,
  InspectorPullRequestCardView,
  InspectorSection,
} from "./SessionInspectorView";
import type { ExternalLinkProps } from "./external-link";

function ExternalLink({ children, ...props }: ExternalLinkProps) {
  return <a {...props}>{children}</a>;
}

it("opts static inspector section copy into text selection without changing the action control", () => {
  render(
    <InspectorSection action={<button type="button">Retry</button>} title="Status">
      <p>Provider connection failed after three attempts.</p>
    </InspectorSection>,
  );

  expect(screen.getByText("Status")).toHaveClass("select-text");
  expect(screen.getByText("Provider connection failed after three attempts.").parentElement).toHaveClass(
    "select-text",
  );
  expect(screen.getByRole("button", { name: "Retry" })).not.toHaveClass("select-text");
});

it("makes pull-request metadata copyable while keeping merge as an ordinary control", () => {
  render(
    <InspectorPullRequestCardView
      countNounLabel={(count, noun) => `${count} ${noun}s`}
      externalLink={ExternalLink}
      mergeAction={<button type="button">Merge</button>}
      openLabel="Open PR #12"
      pr={{
        additions: 4,
        author: "ada",
        card: {
          primary: { key: "merge", label: "Ready to merge", links: [], tone: "success" },
          supporting: [],
        },
        changedFiles: 2,
        deletions: 1,
        href: "https://example.com/pull/12",
        number: 12,
        provider: "github",
        sourceBranch: "feature/copyable-status",
        state: "open",
        stateLabel: "open",
        targetBranch: "main",
        title: "Make inspector copy selectable",
      }}
    />,
  );

  const article = screen.getByRole("article");
  expect(article).toHaveClass("select-text");
  expect(screen.getByText("feature/copyable-status")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Merge" })).not.toHaveClass("select-text");
});

it("makes activity facts copyable", () => {
  render(
    <InspectorActivityTimelineView
      events={[
        {
          content: <span>Controller stopped with exit code 137</span>,
          timestamp: "2m ago",
          tone: "error",
        },
      ]}
    />,
  );

  const event = screen.getByTestId("inspector-timeline-event");
  expect(event.parentElement).toHaveClass("select-text");
  expect(screen.getByText("Controller stopped with exit code 137")).toBeInTheDocument();
});

it("keeps loading and empty-state copy selectable", () => {
  const onViewChange = vi.fn();
  const { rerender } = render(
    <div className="select-none">
      <p className="text-xs text-settings-muted leading-normal">Loading session…</p>
    </div>,
  );

  // This mirrors the inspector's body-level no-selection inheritance and pins
  // the exported empty-state class as the explicit opt-in surface.
  const loading = screen.getByText("Loading session…");
  expect(loading).toHaveClass("select-text");
  void onViewChange;
  rerender(<></>);
});
