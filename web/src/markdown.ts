// Markdown rendering for rule prose (check.Rule.Detail, WS9-020). The output is not sanitized,
// because the source is the rule catalog or an integrator's injected rules, never user input.
import { marked } from "marked";

// RULE_DOC_IMAGE_BASE is the server route for rule-doc diagrams (WS9-030). Detail markdown names
// them by bare filename, e.g. ![reach cases](reach-cases.png), and they resolve against this route.
const RULE_DOC_IMAGE_BASE = "/rule-docs/";

// isResolved is true for a src with a scheme (http:, data:) or a root-absolute path.
function isResolved(src: string): boolean {
  return /^[a-z][a-z0-9+.-]*:/i.test(src) || src.startsWith("/");
}

// Rewrite relative image hrefs to the rule-doc route BEFORE rendering. walkTokens mutates the
// parsed token, which survives changes to marked's renderer signatures. marked.use is global, so
// this registers once at module load and applies to every marked caller.
marked.use({
  walkTokens(token) {
    if (token.type === "image" && token.href && !isResolved(token.href)) {
      token.href = RULE_DOC_IMAGE_BASE + token.href;
    }
  },
});

// renderMarkdown converts markdown to an HTML string for an innerHTML sink. Empty in, empty out,
// so callers can gate the surrounding element on the source.
export function renderMarkdown(md: string): string {
  if (!md) return "";
  return marked.parse(md, { async: false });
}
