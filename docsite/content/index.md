---
title: "Agni"
description: "Check KiCad and EDIF schematics and boards for design mistakes, in the CLI or in your browser, with every finding located on the schematic."
hideTitle: true
---

<div class="home-hero">
<h1>Agni</h1>
<p class="hero-subtitle">Agni checks schematics and boards for design mistakes, such as a bus with no pull-up, a rail with no decoupling, or a connector with no ESD protection, and locates every finding on the schematic. It reads KiCad, EDIF, IPC-2581, xschem and gEDA designs into one representation, then checks, diffs, queries, and renders them.</p>
<div class="hero-actions">
<a href="{{.Site.PathPrefix}}/demo/" class="btn btn-primary">Try it in your browser</a>
<a href="{{.Site.PathPrefix}}/guide/getting-started/" class="btn btn-secondary">Get started</a>
<a href="{{.Site.PathPrefix}}/overview/" class="btn btn-secondary">What is Agni</a>
<a href="https://github.com/panyam/agni" class="btn btn-outline">GitHub</a>
</div>
</div>

<div class="features">
<div class="feature-card">
<h3>Many formats, one IR</h3>
<p>EDIF, KiCad, IPC-2581, xschem, gEDA and Lepton, and Telesis read into a single neutral representation. Checks, diff, query, and rendering are written once and work across every format.</p>
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
</div>

<div class="sections-grid">
<h2>Where to go next</h2>
<div class="section-cards">
<a href="{{.Site.PathPrefix}}/guide/" class="section-card">
<h3>User guide</h3>
<p>Install Agni and run checks, diffs, and queries on a design. For hardware engineers, no Go required.</p>
</a>
<a href="{{.Site.PathPrefix}}/build/" class="section-card">
<h3>Developer guide</h3>
<p>Add a format reader, author a check rule, or write a private extension against the public engine.</p>
</a>
<a href="{{.Site.PathPrefix}}/learn/" class="section-card">
<h3>Learn the domain</h3>
<p>Thirteen chapters teaching what a hardware engineer knows, each ending in the rules that encode it. Written for a software background.</p>
</a>
<a href="{{.Site.PathPrefix}}/architecture/" class="section-card">
<h3>Understand it</h3>
<p>How the IR, net solving, geometry, diff, rules, the datasheet layer and the web app fit together.</p>
</a>
<a href="{{.Site.PathPrefix}}/demo/" class="section-card">
<h3>Try the demo</h3>
<p>Open an example board, or drop your own, in the viewer with the engine running in your browser. Nothing to install and nothing uploaded.</p>
</a>
<a href="{{.Site.PathPrefix}}/demos/" class="section-card">
<h3>About the demo</h3>
<p>What the demo holds, why it runs in the browser, and how it relates to agni serve.</p>
</a>
</div>
</div>
