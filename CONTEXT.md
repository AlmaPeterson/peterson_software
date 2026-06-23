# peterson_software

A catalog and distribution portal for internally-built software: each entry can be downloaded per platform, with access controlled by Visibility and the requester's account.

## Language

### Catalog

**App**:
A cataloged piece of software available for download, identified by a human-readable Name and a stable Slug. Has a Description, a current Version, an optional icon, and one or more Releases.
_Avoid_: Software, Product, Entry

**Release**:
A platform-specific installer file for an App's current Version. A Release has no identity or lifecycle independent of its App — it never exists without one. Its Platform (Android, iOS, Windows, Mac, Linux, Other) is derived from the installer filename, not chosen separately.
_Avoid_: Build, Artifact, File

**Slug**:
The stable identifier used in an App's public download/detail URLs. Assigned once from the App's Name when the App is created, and never changes afterward — even if the Name is later edited, so a Slug and its App's displayed Name can drift apart over time. Unique across all Apps.
_Avoid_: Handle, Key, Identifier

**Visibility**:
Whether an App can be seen by a visitor with no account. A Public App is visible to anyone; a Private App is visible only to someone signed in, regardless of Role.
_Avoid_: is_public, Access level

### Accounts

**User**:
An account holder. Distinct from an App or Release — has no relationship to either beyond performing actions on them.
_Avoid_: Account

**Role**:
A User's standing, one of: Pending (registered, awaiting admin approval, cannot sign in), User (can sign in, and sees Private Apps), or Admin (can additionally create, edit, and delete Apps and Releases, and change other Users' Roles).
_Avoid_: Permission, Access level
