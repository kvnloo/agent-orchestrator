# Frontend worktree bootstrap

Use the repository runtime selector before installing dependencies:

```bash
nvm use                 # or mise/nodenv using .node-version
npm run bootstrap:frontend
```

The command requires Node 24 and npm 11, then prepares these package roots in dependency order:

1. repository root
2. `packages/product-ui`
3. `packages/cloud-client`
4. `frontend`
5. `frontend/src/docs`

Every install uses the committed lockfile and includes optional platform packages. After the desktop frontend install, bootstrap loads Vite and opens an in-memory `better-sqlite3` database. If either native path is missing, it performs one clean reinstall and verifies again before failing with recovery details.

## Fast repeat runs

Bootstrap writes ignored receipts under `.ao/bootstrap/`. A package root is reused only when all of these still match:

- package-lock hash
- Node version
- npm version
- operating system
- CPU architecture

The corresponding `node_modules` directory must also exist. The frontend receipt is accepted only after native verification succeeds. Use `--force` to reinstall every package root:

```bash
npm run bootstrap:frontend -- --force
```

Validate the runtime and all lockfiles without installing anything:

```bash
npm run bootstrap:frontend -- --check
```

These receipts are local to one worktree. They do not share a mutable `node_modules` directory with sibling branches. Cross-worktree snapshot reuse is a separate phase and must retain the same runtime/OS/architecture/lockfile boundaries.
