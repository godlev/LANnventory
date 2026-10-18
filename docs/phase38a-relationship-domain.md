# Phase 38A.1 — Relationship domain foundation

**Scope:** domain contracts, type registry, deterministic directionality, manual
validation and tests. This checkpoint introduces no tables, endpoints, UI,
network collectors, migration, updater release or graph traversal.

## Source-of-truth boundaries

- A Proxmox hypervisor is an existing LAN Host with a hypervisor profile.
- `infrastructure_workloads.HYPERVISOR_MAC` owns the HOSTS association.
- `infrastructure_workload_host_links` owns the MATCHES_HOST association, with
  its existing manual/exact-MAC reconciliation semantics.
- Neither link is copied to a second writable relationship table; Phase 38A.4
  will project both into the canonical graph read model.
- Identity correlation decisions, IP/address reuse, workload match suggestions,
  services and discovery evidence are not topology relationships.
- Only existing Host and Infrastructure Workload entities are supported; never
  synthesize LAN Hosts for VMs or containers.

## Active registry

| Type | Source | Target | Direction | Manual CRUD | Inverse label |
| --- | --- | --- | --- | --- | --- |
| `hosts` | Host | Workload | directed | no | Runs on |
| `matches_host` | Workload | Host | directed | no | Represented by workload |
| `connected_to` | Host | Host | directed | yes | Connected devices |
| `manages` | Host | Host | directed | yes | Managed by |
| `depends_on` | Host | Host | directed | yes | Required by |

All five predicates are directed. Reverse labels are views, never duplicate
inverse records. None is hierarchical; non-hierarchical cycles are valid.
`connected_to` is the user's declared attachment/upstream relationship,
**not** proof of a physical link, port, or topology path.

`uplink_to`, `parent_of`, symmetric predicates and automatic physical discovery
are deliberately not enabled. A future hierarchical type must add transactional
cycle enforcement before enabling writes. A future symmetric predicate must
define endpoint restrictions and use registry-based canonicalization.

## Assertions and provenance

`relationships.Assertion` describes one source claim. In 38A.1 a normal manual
request can only create `connected_to`, `manages` or `depends_on` between
existing Hosts. The constructor assigns `manual` provenance; callers cannot
select `proxmox` or `system`. API JSON decoding in 38A.3 must reject unknown
fields. Proxmox ownership and matches remain controlled by their Phase 35
writers, including manually selected workload matches.

Existing infrastructure claims will later be projected with explicit source
metadata (e.g. `proxmox` + `proxmox-api`/`script-import`, or `system` +
`exact-mac`). These details are not assumed to be interchangeable.

Future network sources can make distinct assertions, keyed by source instance,
without overriding user-managed assertions. No inferred confidence percentages
or effective-edge conflict resolution are introduced now.

## Validation contract

- Registry-allowed endpoint types and nonzero IDs.
- Reject self-relationships and unsupported relationship types.
- Reject generic manual attempts to write HOSTS/MATCHES_HOST.
- Resolve both entities against **current** inventory; old IP/MAC observations
  cannot satisfy existence. Phase 38A.2 must perform this inside a transaction.
- Normalize whitespace in optional notes, bound to 500 Unicode code points,
  reject embedded control characters except newline/tab.
- Canonical identity excludes note, timestamps and UI labels; database uniqueness
  and concurrency-safe duplicate errors are Phase 38A.2 responsibilities.
- Delete/retire cleanup, atomic graph projection and logical backup format
  changes belong to later checkpoints; they are not implemented here.

## 38A.1 tests

`backend/internal/relationships/relationships_test.go` tests endpoint rules,
reserved types, manual provenance, normalization, duplicate identity keys,
reverse labels, resolver errors, note validation and nonhierarchical cycles.

The repository Packaging Check workflow is triggered on development-branch
pushes. No main merge or prerelease is allowed until the required UAT gates.

## Phase 38A.2 — durable assertions

The new `relationship_assertions` table uses a primary key, a seven-field composite
unique index (type, typed source, typed target, provenance, source instance) and
indexes on both typed endpoint pairs. SQLite and PostgreSQL are supported by
the existing GORM migration machinery.

The app exposes only manual Host-to-Host persistence mutations for the three
approved predicates. Source and target IDs refer to current inventory records,
not historical observations or mutable network addresses. Direct create never
upserts an existing assertion. Manual deletion and note updates cannot mutate
a Proxmox/system-owned assertion.

Deletion of a current Host removes both incoming and outgoing assertions in
the enclosing Host transaction; destructive workload deletion similarly
cleans future assertions for its ID. Ordinary offline transitions and workload
retirement do not delete manual assertions.

Logical export **v6** includes all source-owned assertions as a consistently
ordered collection. Missing lists from historical v5 JSON normalize to an
empty collection. **The application has logical export, not a restore importer;**
export compatibility does not imply restore functionality.

No API, UI, visual topology, source collector or main-branch merge is included.
