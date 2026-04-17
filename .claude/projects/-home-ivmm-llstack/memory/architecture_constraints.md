---
name: Architecture Constraints
description: Non-negotiable architecture rules that must never be violated
type: feedback
---

1. Apache VirtualHost semantics = single source of truth
2. OLS config compiled from canonical model, never user-maintained
3. All writes: plan -> apply -> verify -> rollback metadata
4. System commands via unified executor, no scattered shell calls
5. Apache PHP = php-fpm (not php-litespeed)
6. OLS/LSWS PHP = Remi php-litespeed (not official lsphp)
7. Default site root: /data/www/<site>
8. No Web GUI - CLI+TUI only
9. All changes must sync dev_docs/

**Why:** These are foundational product decisions established in Phase 0. Violating any would break the canonical model architecture.

**How to apply:** Before any code change, verify it doesn't conflict with these rules. Check ADRs and ARCHITECTURE.md when in doubt.
