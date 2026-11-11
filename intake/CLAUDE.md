# intake

This file covers the sanitized design summary that `intake/` produces.

- **The confidentiality guarantee is structural.** The `Skeleton` type has no field that can hold a net name or a connection, so an intake summary cannot express the confidential parts of a design. The policy "do not paste a net name" becomes a property of the type.
- **Never add a free-text field to `Skeleton`.** One such field would dissolve the guarantee.
