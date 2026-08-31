import type { DirectoryListing } from './api/client';

// Workspace listings now come exclusively from GET /api/listing. Keeping this
// module prevents old source imports from silently reviving shell-injected data.
export function getWorkspaceRuntime(): DirectoryListing {
  throw new Error('Workspace runtime injection has been removed; use api.getListing instead.');
}
