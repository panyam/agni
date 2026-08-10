---
title: "Design decisions"
description: "The rationale behind the larger architectural choices."
---

This section records the reasoning behind Agni's larger choices, in the voice of a decision
record: what was chosen, what was rejected, and why. It is separate from the architecture
reference on purpose. The architecture pages describe how things work today. These pages
explain why they are that way.

- **[Open core](open-core/)**: a public Apache-2.0 engine and a private overlay that depends on
  it, and why the license is Apache rather than GPL.

<!-- Parked. To publish this section again: move it to content/decisions/, move DecisionsNav.html
     back to templates/nav/, add BOTH the include and a dispatch branch in templates/Sidebar.html,
     and add an entry to content/HeaderNavLinks.json. nav_test.go enforces all four. -->
