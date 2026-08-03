// Markdown rendering for rule prose (WS9-020). The catalog's Detail travels as markdown
// (check.Rule.Detail); rendering it is a view concern, so the client owns the conversion. The
// content is authored in-repo (the rule catalog, or an integrator's injected rules), not user
// input, so the output is trusted and not sanitized.
import { marked } from "marked";

// RULE_DOC_IMAGE_BASE is where the server serves the rule-doc explainer diagrams (WS9-030). A rule's
// Detail markdown references them by bare filename (e.g. ![reach cases](reach-cases.png)), authored
// relative to the doc; the panels resolve those against this route so the diagrams show inline.
const RULE_DOC_IMAGE_BASE = "/rule-docs/";

// A src is already resolvable when it has a scheme (http:, data:) or is root-absolute; only a
// relative filename needs the rule-doc base prepended.
function isResolved(src: string): boolean {
  return /^[a-z][a-z0-9+.-]*:/i.test(src) || src.startsWith("/");
}

// Rewrite each relative image href to the rule-doc route BEFORE rendering, so the default renderer
// emits the resolved <img src>. walkTokens mutates the parsed token, which is version-robust across
// marked's renderer signatures. Registered once at module load (marked.use is global).
marked.use({
  walkTokens(token) {
    if (token.type === "image" && token.href && !isResolved(token.href)) {
      token.href = RULE_DOC_IMAGE_BASE + token.href;
    }
  },
});

// renderMarkdown converts markdown source to an HTML string for an innerHTML sink. Empty in,
// empty out, so callers can gate the surrounding element on the source. Relative image refs
// resolve to the rule-doc route (WS9-030).
export function renderMarkdown(md: string): string {
  if (!md) return "";
  return marked.parse(md, { async: false });
}
