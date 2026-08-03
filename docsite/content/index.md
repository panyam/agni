---
title: "Agni"
description: "An EDA tooling engine that reads schematics and boards from many formats into one neutral IR, then checks, diffs, queries, and renders them."
hideTitle: true
---

<div class="home-hero">
<h1>Agni</h1>
<p class="hero-subtitle">A project for learning hardware design by building tooling for it. Agni reads schematics and boards from several EDA formats into one representation, then lets you check, diff, query, and render them.</p>
<div class="hero-actions">
<a href="{{.Site.PathPrefix}}/guide/getting-started/" class="btn btn-primary">Get started</a>
<a href="{{.Site.PathPrefix}}/overview/" class="btn btn-secondary">What is Agni</a>
<a href="https://github.com/panyam/agni" class="btn btn-outline">GitHub</a>
</div>
</div>

<div class="features">
<div class="feature-card">
<h3>Many formats, one IR</h3>
<p>EDIF, KiCad, and IPC-2581 read into a single neutral representation. Checks, diff, query, and rendering are written once and work across every format.</p>
<a href="{{.Site.PathPrefix}}/architecture/ingestion-and-ir/">How ingestion works &rarr;</a>
</div>
<div class="feature-card">
<h3>Checks and reports</h3>
<p>Run a catalog of electrical and integrity rules over a design. Every finding cites its subject and its evidence.</p>
<a href="{{.Site.PathPrefix}}/guide/checks-and-reports/">Run checks &rarr;</a>
</div>
<div class="feature-card">
<h3>Query a design as data</h3>
<p>Ask datalog questions about nets, parts, copper, and datasheet limits. Each answer is a cited row you can click to locate.</p>
<a href="{{.Site.PathPrefix}}/guide/querying/">Query your design &rarr;</a>
</div>
<div class="feature-card">
<h3>Open core</h3>
<p>The engine is open source under Apache-2.0. Private, house-specific work lives in an overlay that depends on it.</p>
<a href="{{.Site.PathPrefix}}/decisions/open-core/">The open-core split &rarr;</a>
</div>
</div>

<div class="sections-grid">
<h2>Where to go next</h2>
<div class="section-cards">
<a href="{{.Site.PathPrefix}}/guide/" class="section-card">
<h3>Use it</h3>
<p>Install Agni and run checks, diffs, and queries on a design. For hardware engineers, no Go required.</p>
</a>
<a href="{{.Site.PathPrefix}}/build/" class="section-card">
<h3>Build on it</h3>
<p>Add a format reader, author a check rule, or write a private overlay against the public engine.</p>
</a>
<a href="{{.Site.PathPrefix}}/architecture/" class="section-card">
<h3>Understand it</h3>
<p>How the pieces fit: the IR, net solving, geometry, diff, rules, the datasheet layer, and the web app.</p>
</a>
<a href="{{.Site.PathPrefix}}/demos/" class="section-card">
<h3>Demos</h3>
<p>Try the query engine and the viewer in your browser, running against a seeded corpus.</p>
</a>
</div>
</div>
