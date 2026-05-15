# Product Narrative Review

Use this document to review Doug as a product story before reviewing it as an implementation.

## Goal

Decide whether Doug still tells a coherent story as a useful tool, or whether the product surface has drifted into accidental complexity.

This review is intentionally narrative-first:

- do not start from commands
- do not start from folders
- do not start from implementation detail
- start from the user problem, the core promise, and the minimum useful loop

---

## Review Rules

During this review, prefer these lenses:

1. **User value over mechanism**
2. **Default workflow over all supported workflows**
3. **Core promise over compatibility baggage**
4. **Coherent story over feature completeness**

Try to describe Doug as if it were a brand-new tool being introduced for the first time.

---

## Part 1: One-Sentence Product Definition

Complete all three, then choose the best one.

### Candidate A

Doug is:

> 

### Candidate B

Doug is:

> 

### Candidate C

Doug is:

> 

### Chosen version

> 

### Notes

- What makes this sentence clear?
- What makes it feel bloated or dishonest?
- Which words feel like implementation leakage?

---

## Part 2: User and Problem

### Primary user

Who is Doug for?

> 

### Problem statement

What painful or messy thing does Doug solve?

> 

### Before Doug

What does the user do without Doug?

> 

### After Doug

What is simpler, safer, or more predictable with Doug?

> 

---

## Part 3: The Irreducible Core Loop

What is the smallest useful end-to-end Doug workflow?

List only the steps that must exist.

1. 
2. 
3. 
4. 
5. 

### Core loop narrative

In plain English:

> 

### Test

If everything else were removed, would this still be a useful product?

- [ ] Yes
- [ ] No

Why?

> 

---

## Part 4: Product Boundaries

### What Doug should own

List the things that are central to the promise.

- 
- 
- 

### What Doug should not own

List the things that should be out of scope, hidden, or delegated.

- 
- 
- 

### What feels suspiciously exposed today

List concepts that may be implementation detail masquerading as product.

- 
- 
- 

---

## Part 5: Workflow Inventory

For each workflow or subsystem, classify it.

| Workflow / concept | Core | Optional | Transitional | Unclear | Notes |
|---|---|---|---|---|---|
| init |  |  |  |  |  |
| run |  |  |  |  |  |
| plan |  |  |  |  |  |
| handoff |  |  |  |  |  |
| scaffold |  |  |  |  |  |
| revert |  |  |  |  |  |
| research |  |  |  |  |  |
| changelog updates |  |  |  |  |  |
| KB updates |  |  |  |  |  |
| backlog epics |  |  |  |  |  |
| Pi RPC transport |  |  |  |  |  |
| subprocess transport |  |  |  |  |  |
| ACTIVE_TASK.md contract |  |  |  |  |  |

Add rows as needed.

---

## Part 6: The Default Story

Describe the default Doug story in 5-7 bullets.

1. 
2. 
3. 
4. 
5. 

### Stress test

Would a new user need to understand any of the following to get value?

- planning vs runtime workspace split
- transport abstraction
- backlog promotion lifecycle
- KB synthesis
- manifest-driven scaffolding
- result archive layout

For each item above, mark:

- **Must know**
- **Should not need to know**

Notes:

> 

---

## Part 7: README Smell Check

Mark any symptom that feels true today.

- [ ] The README explains too many modes before the happy path
- [ ] The README mixes user value with implementation mechanics
- [ ] The README describes multiple overlapping mental models
- [ ] The README defends compatibility instead of selling usefulness
- [ ] The README requires too much vocabulary before the tool makes sense
- [ ] The README makes the tool feel larger than the problem it solves
- [ ] The README cannot explain the default workflow without caveats

### Evidence

Quote or summarize the sections that triggered the smell.

> 

---

## Part 8: Reduction Exercise

### A. Keep

What absolutely belongs in the product story?

- 
- 
- 

### B. Simplify

What should remain, but in a much thinner form?

- 
- 
- 

### C. Hide

What may be real internally, but should not be front-and-center in the narrative?

- 
- 
- 

### D. Cut

What feels like baggage, drift, or a second product?

- 
- 
- 

---

## Part 9: Rewrite the Pitch

Write the README opening you wish Doug had.

### Draft

> 

### Constraints

The opening should:

- say what Doug is in plain language
- identify the default user and use case
- explain the happy path before advanced flows
- avoid implementation detail unless essential
- avoid listing every supported mode

---

## Part 10: Outcome

### Current diagnosis

Choose one:

- [ ] The product is coherent; the docs are the main problem
- [ ] The product is coherent, but the surface area is too exposed
- [ ] The product has one strong core plus too many side systems
- [ ] The product story is fragmented because the product is fragmented

### Top 3 simplification moves

1. 
2. 
3. 

### What to change next

- [ ] Rewrite README
- [ ] Shrink visible product surface
- [ ] Retire transitional workflows
- [ ] Collapse overlapping concepts
- [ ] Re-scope Doug around a smaller core promise

### Notes

> 
