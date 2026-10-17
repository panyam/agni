// FindingFeedback is everything a "this looks wrong" report says about a finding (agni issues 858,
// 1057): the rule's name when agni ships it, its severity, whether it was inconclusive, and the
// engine's version. It has no field for the finding's subject, pin, message, context or sheets, or
// for a rule a project wrote, whose name is the project's, so the issue it prefills cannot carry any
// of them however it is built. The CLI's trial report (Go, feedback.Report) holds the same line.
export interface FindingFeedback {
  // rule is the shipped rule's name, "" for a rule the design's project or an overlay composed.
  rule: string;
  severity: string;
  inconclusive: boolean;
  version: string;
}

// findingFeedback takes what a report may say from a finding, naming its rule only when shipped
// says agni ships it.
export function findingFeedback(f: { rule: string; severity: string; inconclusive: boolean }, shipped: boolean, version: string): FindingFeedback {
  return { rule: shipped ? f.rule : "", severity: f.severity, inconclusive: f.inconclusive, version };
}

// feedbackIssueURL opens a new issue on repo prefilled from a FindingFeedback, for the reader to add
// what they expected and submit.
export function feedbackIssueURL(fb: FindingFeedback, repo = "panyam/agni"): string {
  const rule = fb.rule ? `\`${fb.rule}\`` : "a rule my project wrote";
  // The severity is named as a value, since an article before it read "a error".
  const how = `severity ${fb.severity}${fb.inconclusive ? ", inconclusive" : ""}`;
  const body = [
    "<!-- This names only agni's rule, the finding's severity and the engine version, nothing from your design. Describe what you expected without naming nets, parts or files. -->",
    "",
    `A finding from ${rule} (${how}) looks wrong on a design I can't share.`,
    "",
    `agni ${fb.version || "(version unknown)"}`,
    "",
    "**What I expected instead:**",
    "",
  ].join("\n");
  const title = fb.rule ? `A ${fb.rule} finding looks wrong` : "A project rule's finding looks wrong";
  const q = new URLSearchParams({ title, body, labels: "finding-feedback" });
  return `https://github.com/${repo}/issues/new?${q.toString()}`;
}
