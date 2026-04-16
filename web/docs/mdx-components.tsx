import { useMDXComponents as getThemeComponents } from "nextra-theme-docs";
import { Mermaid } from "./src/components/mermaid";

const themeComponents = getThemeComponents();

// Wrap the default `pre` component to intercept mermaid code blocks and
// render them as diagrams instead of syntax-highlighted code.
function Pre(props: React.ComponentProps<"pre">) {
  const child = props.children as React.ReactElement<{
    className?: string;
    children?: string;
  }> | undefined;
  if (
    child &&
    typeof child === "object" &&
    "props" in child &&
    child.props?.className === "language-mermaid"
  ) {
    return <Mermaid chart={String(child.props.children).trim()} />;
  }
  const ThemePre = themeComponents.pre ?? "pre";
  return <ThemePre {...props} />;
}

export function useMDXComponents(components: Record<string, React.ComponentType> = {}) {
  return {
    ...themeComponents,
    pre: Pre,
    ...components,
  };
}
