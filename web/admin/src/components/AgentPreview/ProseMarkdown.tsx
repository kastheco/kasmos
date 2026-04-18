import type { ComponentProps } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

// ---------------------------------------------------------------------------
// Component renderers
//
// Raw HTML passthrough is NOT enabled — react-markdown's default behaviour
// already strips raw HTML elements.  No allowDangerousHtml.
//
// The code-block treatment matches TaskDetailPage.tsx: fenced code gets a
// language label above the <pre> so it stays consistent with the rest of the
// admin UI.
// ---------------------------------------------------------------------------

const mdComponents: ComponentProps<typeof ReactMarkdown>["components"] = {
  pre({ node: _node, children, ...rest }) {
    const child = Array.isArray(children) ? children[0] : children;
    const className =
      child && typeof child === "object" && "props" in child
        ? (child.props as { className?: string }).className ?? ""
        : "";
    const match = /language-(\w+)/.exec(className);
    if (match) {
      return (
        <div style={{ position: "relative", marginBottom: "0.5em" }}>
          <span
            style={{
              display: "block",
              fontSize: "10px",
              color: "var(--rp-muted)",
              fontFamily: "var(--font-mono)",
              marginBottom: "2px",
              textTransform: "lowercase",
            }}
          >
            {match[1]}
          </span>
          <pre {...rest}>{children}</pre>
        </div>
      );
    }
    return <pre {...rest}>{children}</pre>;
  },
  code({ node: _node, className, children, ...rest }) {
    return (
      <code className={className} {...rest}>
        {children}
      </code>
    );
  },
  a({ node: _node, href, children, ...rest }) {
    const isExternal =
      href?.startsWith("http://") || href?.startsWith("https://");
    if (isExternal) {
      return (
        <a href={href} target="_blank" rel="noopener noreferrer" {...rest}>
          {children}
        </a>
      );
    }
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  },
};

// ---------------------------------------------------------------------------
// ProseMarkdown
// ---------------------------------------------------------------------------

interface ProseMarkdownProps {
  text: string;
}

/**
 * Renders agent prose output as Markdown using react-markdown + remark-gfm.
 * Raw HTML passthrough is disabled (default).
 * External links open in a new tab with rel="noopener noreferrer".
 */
export function ProseMarkdown({ text }: ProseMarkdownProps) {
  return (
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
      {text}
    </ReactMarkdown>
  );
}
