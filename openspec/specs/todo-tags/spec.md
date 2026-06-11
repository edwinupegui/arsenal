# todo-tags

## Purpose

Defines tag attachment and normalization for todos, reusing the cross-domain tag namespace and the `domain.WithTags` helper already proven in resources. Tags are shared across all domains — a tag like `"urgente"` can apply to both a todo and a resource, pointing to the same row in the `tags` table.

## Requirements

### Requirement: Tag normalization

The system MUST normalize all tag inputs through `domain.NormalizeTags` before attachment. This lowercases, deduplicates, sorts, and drops empty entries. Tags exceeding the max length SHALL be rejected.

#### Scenario: Dedup repeated tags

- **WHEN** tags `["Urgente", "urgente", "URGENTE"]` are provided
- **THEN** after normalization, only `["urgente"]` is attached

#### Scenario: Drop empty tags silently

- **WHEN** tags `["work", "", "  "]` are provided
- **THEN** after normalization, only `["work"]` is attached (empties dropped without error)

### Requirement: Attach via WithTags

The system MUST attach tags to a todo via `domain.WithTags` using a todo-specific `Attacher` adapter that satisfies the `domain.Attacher` interface. The adapter translates the generic `OwnerID` into `todo_id`. When `pruneOrphans=true`, tags no longer referenced by any owner across all domains SHALL be deleted from the `tags` table.

#### Scenario: Attach new tags to a todo

- **WHEN** tags `["casa", "luz"]` are attached to todo ID 5
- **THEN** `todo_tags` contains rows `(5, tag_id_of_casa)` and `(5, tag_id_of_luz)`
- **AND** the `tags` table contains rows for `"casa"` and `"luz"` (upserted)

#### Scenario: Prune orphans when removing a tag

- **WHEN** a todo's tags are updated from `["a", "b"]` to `["a"]` with `pruneOrphans=true`
- **AND** no other owner (resource, todo, etc.) references tag `"b"`
- **THEN** tag `"b"` is deleted from the `tags` table

### Requirement: Shared tag namespace

The system MUST reuse the same `tags` table as resources. A tag created by a todo and a tag created by a resource with the same name SHALL resolve to the same `tags` row.

#### Scenario: Share tag between todo and resource

- **WHEN** a resource has tag `"importante"` and a todo is created with tag `"importante"`
- **THEN** both the `resource_tags` and `todo_tags` junction tables reference the same `tags` row
- **AND** a search for tag `"importante"` across domains returns both the resource and the todo

## Out of Scope

- Tag hierarchies, namespaces, or scoping per domain (global namespace by design).
- Tag colors, icons, or metadata beyond the name.
- Tag autocomplete or suggestions in the service layer (UI concern).
- Bulk tag operations (rename a tag across all domains).
