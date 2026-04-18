import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ProseMarkdown } from "./ProseMarkdown";

describe("ProseMarkdown", () => {
  it("renders bold text as <strong>", () => {
    render(<ProseMarkdown text="This is **bold** text." />);
    const strong = document.querySelector("strong");
    expect(strong).toBeTruthy();
    expect(strong?.textContent).toBe("bold");
  });

  it("renders a link with href", () => {
    render(<ProseMarkdown text="See [docs](https://example.com) here." />);
    const a = document.querySelector("a");
    expect(a).toBeTruthy();
    expect(a?.getAttribute("href")).toBe("https://example.com");
  });

  it("adds target=_blank and rel=noopener noreferrer on external links", () => {
    render(<ProseMarkdown text="[link](https://example.com)" />);
    const a = document.querySelector("a");
    expect(a?.getAttribute("target")).toBe("_blank");
    expect(a?.getAttribute("rel")).toBe("noopener noreferrer");
  });

  it("does not add target=_blank on relative links", () => {
    render(<ProseMarkdown text="[link](/internal)" />);
    const a = document.querySelector("a");
    expect(a?.getAttribute("target")).toBeNull();
  });

  it("does not render raw HTML tags", () => {
    render(<ProseMarkdown text="<script>alert('xss')</script> safe text" />);
    // react-markdown strips raw HTML by default (no allowDangerousHtml)
    expect(document.querySelector("script")).toBeNull();
    expect(screen.getByText(/safe text/)).toBeTruthy();
  });

  it("renders plain text unchanged", () => {
    render(<ProseMarkdown text="Hello world" />);
    expect(screen.getByText("Hello world")).toBeTruthy();
  });

  it("renders inline code", () => {
    render(<ProseMarkdown text="Use `npm install` to install." />);
    const code = document.querySelector("code");
    expect(code).toBeTruthy();
    expect(code?.textContent).toBe("npm install");
  });
});
