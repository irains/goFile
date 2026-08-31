import { Suspense, lazy, useEffect, useState } from 'react';
import { Add, Archive, ContentCopy, CreateNewFolder, Delete, Download, Edit, Folder, FolderOpen, MoreVert, Refresh, UploadFile } from '@mui/icons-material';
import { Alert, AppBar, Avatar, Box, Breadcrumbs, Button, Checkbox, Chip, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle, Divider, Drawer, IconButton, List, ListItemButton, ListItemIcon, ListItemText, Menu, MenuItem, Paper, Snackbar, Stack, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TextField, Toolbar, Tooltip, Typography, useMediaQuery } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError, type FileEntry, type Properties } from '../api/client';
import { itemUrl } from '../runtime';
import { useI18n } from '../i18n';
import { useSession } from '../session/SessionProvider';
import { UploadQueueDrawer } from './UploadQueueDrawer';

const LazyEditorDialog = lazy(() => import('../editor/EditorDialog').then((module) => ({ default: module.EditorDialog })));
type FormAction = 'newdir' | 'newfile' | 'rename' | 'move' | 'copy';
type FormState = { action: FormAction; entry?: FileEntry } | null;

const fmtBytes = (value: number) => new Intl.NumberFormat(undefined, { style: 'unit', unit: 'byte', notation: value > 1_000_000 ? 'compact' : 'standard', unitDisplay: 'narrow' }).format(value);
const fmtDate = (value: string) => value ? new Date(value).toLocaleString() : '—';
const navigateDirectory = (path: string) => { window.location.assign(itemUrl('d', path)); };

function directoryPathFromLocation(): string {
  const base = itemUrl('d', '').replace(/\/$/, '');
  const pathname = window.location.pathname.replace(/\/$/, '');
  if (pathname === base || pathname === `${base}/d`) return '';
  const prefix = `${base}/d/`;
  if (!pathname.startsWith(prefix)) return '';
  return pathname.slice(prefix.length).split('/').filter(Boolean).map(decodeURIComponent).join('/');
}

export function editorPathFromLocation(location: Pick<Location, 'pathname'> = window.location): string | null {
  const base = itemUrl('d', '').replace(/\/$/, '');
  const pathname = location.pathname.replace(/\/$/, '');
  const prefix = `${base}/edit/`;
  if (!pathname.startsWith(prefix)) return null;
  try {
    const value = pathname.slice(prefix.length).split('/').filter(Boolean).map(decodeURIComponent).join('/');
    return value || null;
  } catch {
    return null;
  }
}

export function Workspace() {
  const { t, locale, setLocale } = useI18n();
  const { session, logout } = useSession();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [menuEntry, setMenuEntry] = useState<FileEntry | null>(null);
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [form, setForm] = useState<FormState>(null);
  const [propertiesFor, setPropertiesFor] = useState<FileEntry | null>(null);
  const [editorFor, setEditorFor] = useState<FileEntry | null>(null);
  const editorPath = editorPathFromLocation();
  const [showUploads, setShowUploads] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const compact = useMediaQuery('(max-width: 760px)');
  const currentPath = directoryPathFromLocation();
  const listingQuery = useQuery({ queryKey: ['listing', currentPath], queryFn: () => api.getListing(currentPath) });
  const listing = listingQuery.data;
  const entries = listing?.entries ?? [];
  const routeEditor = editorPath ? {
    name: editorPath.split('/').pop() ?? editorPath,
    path: editorPath,
    kind: 'file' as const,
    sizeBytes: 0,
    modifiedAt: '',
    mode: '',
    extension: '',
    isArchive: false,
    previewable: false,
    version: ''
  } : null;
  const activeEditor = editorFor ?? routeEditor;
  const mutable = Boolean(session?.capabilities.mutate);
  const canUpload = Boolean(session?.capabilities.upload);

  const refresh = async () => {
    setSelected(new Set());
    await queryClient.invalidateQueries({ queryKey: ['listing', currentPath] });
  };
  const mutation = useMutation({
    mutationFn: ({ endpoint, values }: { endpoint: string; values: Record<string, string> }) => api.mutate<{ ok: true; hash?: string }>(endpoint, values),
    onSuccess: async (result) => { if (result.hash) setMessage(result.hash); await refresh(); },
    onError: (error) => setMessage(error instanceof ApiError ? t(`error.${error.code}`) : t('error.generic'))
  });
  const batch = useMutation({
    mutationFn: ({ endpoint, body }: { endpoint: string; body: unknown }) => api.batch<{ ok: boolean; download_url?: string }>(endpoint, body),
    onSuccess: async (result) => { if (result.download_url) window.location.assign(result.download_url); else await refresh(); },
    onError: (error) => setMessage(error instanceof ApiError ? t(`error.${error.code}`) : t('error.generic'))
  });
  const propertyQuery = useQuery({ queryKey: ['properties', propertiesFor?.path], queryFn: () => api.getProperties(propertiesFor!.path), enabled: Boolean(propertiesFor) });

  useEffect(() => { setSelected(new Set()); }, [listing?.listingToken]);
  const select = (entry: FileEntry, checked: boolean) => setSelected((previous) => { const next = new Set(previous); if (checked) next.add(entry.path); else next.delete(entry.path); return next; });
  const selectedEntries = entries.filter((entry) => selected.has(entry.path));
  const batchBody = (destination?: string) => ({ listing_token: listing?.listingToken, entries: selectedEntries.map(({ name, version }) => ({ name, version })), ...(destination !== undefined ? { destination } : {}) });
  const doBatch = (endpoint: string, destination?: string) => { if (listing?.listingToken && selectedEntries.length) batch.mutate({ endpoint, body: batchBody(destination) }); };
  const performEntryAction = (action: string, entry: FileEntry) => {
    setMenuEntry(null); setMenuAnchor(null);
    if (action === 'properties') return setPropertiesFor(entry);
    if (action === 'edit') return setEditorFor(entry);
    if (action === 'download') return window.location.assign(itemUrl('download', entry.path));
    if (action === 'preview') return window.location.assign(itemUrl('view', entry.path));
    if (action === 'rename') return setForm({ action: 'rename', entry });
    if (action === 'move' || action === 'copy') return setForm({ action, entry });
    const endpoints: Record<string, string> = { delete: 'do/rm', archive: 'do/zip', extract: 'do/unzip', checksum: 'do/md5' };
    if (action === 'delete' && !window.confirm(t('dialog.deleteText'))) return;
    mutation.mutate({ endpoint: endpoints[action], values: { path: entry.path } });
  };
  const signOut = async () => {
    try { await logout(); window.location.assign(itemUrl('d', '')); }
    catch (error) { setMessage(error instanceof ApiError ? t(`error.${error.code}`) : t('error.generic')); }
  };

  if (listingQuery.isLoading) return <Stack component="main" alignItems="center" justifyContent="center" sx={{ minHeight: '100dvh' }}><CircularProgress /></Stack>;
  if (listingQuery.isError || !listing) return <Box component="main" sx={{ p: 4 }}><Alert severity="error">{listingQuery.error instanceof ApiError ? t(`error.${listingQuery.error.code}`) : t('error.generic')}</Alert><Button sx={{ mt: 2 }} onClick={() => void listingQuery.refetch()}>{t('workspace.refresh')}</Button></Box>;

  return <Box component="main" sx={{ minHeight: '100dvh', bgcolor: 'background.default' }}>
    <AppBar position="sticky" elevation={0} color="transparent" sx={{ borderBottom: '1px solid', borderColor: 'divider', bgcolor: 'background.default' }}>
      <Toolbar sx={{ gap: 1.5, px: { xs: 2, sm: 3 } }}>
        <Avatar variant="rounded" sx={{ bgcolor: 'primary.main', color: 'primary.contrastText', width: 32, height: 32, fontWeight: 900 }}>F</Avatar>
        <Typography fontWeight={800} sx={{ mr: 'auto' }}>FileHarbor</Typography>
        <Tooltip title={t('workspace.refresh')}><IconButton aria-label={t('workspace.refresh')} onClick={() => void refresh()}><Refresh /></IconButton></Tooltip>
        <Button size="small" onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}>{locale === 'en' ? '中文' : 'EN'}</Button>
        <Button variant="text" onClick={() => void signOut()}>{t('app.signOut')}</Button>
      </Toolbar>
    </AppBar>
    <Box sx={{ maxWidth: 1440, mx: 'auto', px: { xs: 2, sm: 3 }, py: 3 }}>
      <Stack spacing={2.5}>
        <Box sx={{ display: 'flex', alignItems: { sm: 'center' }, flexDirection: { xs: 'column', sm: 'row' }, gap: 2 }}>
          <Box sx={{ flex: 1, minWidth: 0, alignSelf: 'stretch' }}>
            <Typography component="h1" variant="h5" fontWeight={800}>{t('app.workspace')}</Typography>
            <Breadcrumbs aria-label="breadcrumb" sx={{ mt: .5, overflow: 'hidden' }}>
              <Button onClick={() => navigateDirectory('')} size="small">{t('workspace.root')}</Button>
              {listing.path.split('/').filter(Boolean).map((segment, index, all) => <Button onClick={() => navigateDirectory(all.slice(0, index + 1).join('/'))} size="small" key={`${segment}-${index}`}>{segment}</Button>)}
            </Breadcrumbs>
          </Box>
          {!mutable && canUpload && <Chip label={t('workspace.uploadsOnly')} color="info" />}
          {!mutable && !canUpload && <Chip label={t('workspace.readOnly')} />}
          {mutable && <Stack direction="row" flexWrap="wrap" gap={1}>
            <Button startIcon={<CreateNewFolder />} variant="outlined" onClick={() => setForm({ action: 'newdir' })}>{t('workspace.newFolder')}</Button>
            {!compact && <Button startIcon={<Add />} variant="outlined" onClick={() => setForm({ action: 'newfile' })}>{t('workspace.newFile')}</Button>}
          </Stack>}
          {canUpload && <Button startIcon={<UploadFile />} variant="contained" onClick={() => setShowUploads(true)}>{t('workspace.upload')}</Button>}
        </Box>
        {listing.truncated && <Alert severity="warning">{t('workspace.truncated')}</Alert>}
        {selectedEntries.length > 0 && <Paper elevation={0} sx={{ border: '1px solid', borderColor: 'divider', p: 1.25, display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
          <Typography fontWeight={700} sx={{ mr: 1 }}>{t('workspace.selected', { count: selectedEntries.length })}</Typography>
          {mutable && <Button size="small" startIcon={<Folder />} onClick={() => setForm({ action: 'move' })}>{t('action.move')}</Button>}
          {mutable && <Button size="small" startIcon={<ContentCopy />} onClick={() => setForm({ action: 'copy' })}>{t('action.copy')}</Button>}
          {mutable && <Button size="small" color="error" startIcon={<Delete />} onClick={() => { if (window.confirm(t('dialog.deleteText'))) doBatch('do/batch/delete'); }}>{t('action.delete')}</Button>}
          <Button size="small" startIcon={<Archive />} onClick={() => doBatch('do/batch/download-zip')}>{t('action.batchDownload')}</Button>
          <Button size="small" onClick={() => setSelected(new Set())}>{t('action.cancel')}</Button>
        </Paper>}
        <TableContainer component={Paper} elevation={0} sx={{ border: '1px solid', borderColor: 'divider' }}>
          <Table stickyHeader size={compact ? 'small' : 'medium'} aria-label={t('app.workspace')}>
            <TableHead><TableRow><TableCell padding="checkbox"><Checkbox aria-label={t('workspace.selectAll')} checked={entries.length > 0 && selected.size === entries.length} indeterminate={selected.size > 0 && selected.size < entries.length} onChange={(event) => setSelected(event.target.checked ? new Set(entries.map((entry) => entry.path)) : new Set())} /></TableCell><TableCell>{t('workspace.name')}</TableCell>{!compact && <TableCell>{t('workspace.size')}</TableCell>}{!compact && <TableCell>{t('workspace.modified')}</TableCell>}<TableCell align="right">{t('workspace.actions')}</TableCell></TableRow></TableHead>
            <TableBody>
              {listing.parentPath !== null && <TableRow hover><TableCell padding="checkbox" /><TableCell colSpan={compact ? 2 : 4}><Button startIcon={<FolderOpen />} onClick={() => navigateDirectory(listing.parentPath!)} sx={{ justifyContent: 'flex-start' }}>..</Button></TableCell></TableRow>}
              {entries.length ? entries.map((entry) => <TableRow hover key={entry.path} selected={selected.has(entry.path)}>
                <TableCell padding="checkbox"><Checkbox checked={selected.has(entry.path)} onChange={(event) => select(entry, event.target.checked)} /></TableCell>
                <TableCell sx={{ maxWidth: 0 }}><Stack direction="row" spacing={1.25} alignItems="center"><ListItemIcon sx={{ minWidth: 28, color: entry.kind === 'directory' ? 'primary.light' : 'text.secondary' }}>{entry.kind === 'directory' ? <Folder /> : <FolderOpen />}</ListItemIcon><Box sx={{ minWidth: 0 }}><Typography component="button" type="button" onClick={() => entry.kind === 'directory' ? navigateDirectory(entry.path) : window.location.assign(itemUrl('download', entry.path))} color="inherit" fontWeight={entry.kind === 'directory' ? 700 : 500} sx={{ appearance: 'none', background: 'none', border: 0, p: 0, textAlign: 'left', cursor: 'pointer', font: 'inherit', '&:hover': { textDecoration: 'underline' }, overflowWrap: 'anywhere' }}>{entry.name}</Typography>{compact && <Typography variant="caption" color="text.secondary">{entry.kind === 'file' ? fmtBytes(entry.sizeBytes) : t('properties.type')} · {fmtDate(entry.modifiedAt)}</Typography>}</Box></Stack></TableCell>
                {!compact && <TableCell>{entry.kind === 'file' ? fmtBytes(entry.sizeBytes) : '—'}</TableCell>}{!compact && <TableCell>{fmtDate(entry.modifiedAt)}</TableCell>}
                <TableCell align="right"><IconButton aria-label={`${t('workspace.actions')} ${entry.name}`} onClick={(event) => { setMenuEntry(entry); setMenuAnchor(event.currentTarget); }}><MoreVert /></IconButton></TableCell>
              </TableRow>) : listing.parentPath === null && <TableRow><TableCell colSpan={compact ? 3 : 5}><Box sx={{ py: 8, textAlign: 'center' }}><FolderOpen color="disabled" sx={{ fontSize: 44 }} /><Typography color="text.secondary">{t('workspace.empty')}</Typography></Box></TableCell></TableRow>}
            </TableBody>
          </Table>
        </TableContainer>
      </Stack>
    </Box>
    <EntryMenu entry={menuEntry} anchor={menuAnchor} onClose={() => { setMenuEntry(null); setMenuAnchor(null); }} onAction={performEntryAction} mutable={mutable} editorAvailable={Boolean(session?.capabilities.editorSave || session?.capabilities.browse)} />
    <EntryForm state={form} currentPath={listing.path} selectedEntries={selectedEntries} onClose={() => setForm(null)} onSubmit={(endpoint, values) => mutation.mutate({ endpoint, values })} onBatchSubmit={doBatch} />
    <PropertiesDialog entry={propertiesFor} properties={propertyQuery.data?.properties} isLoading={propertyQuery.isLoading} onClose={() => setPropertiesFor(null)} />
    {activeEditor && <Suspense fallback={<CircularProgress />}><LazyEditorDialog entry={activeEditor} writable={Boolean(session?.capabilities.editorSave)} onClose={() => { setEditorFor(null); if (editorPath) window.location.assign(itemUrl('d', '')); }} /></Suspense>}
    {canUpload && <UploadQueueDrawer open={showUploads} onClose={() => setShowUploads(false)} destination={listing.path} username={session?.username ?? ''} onAllComplete={() => void refresh()} />}
    <Snackbar open={Boolean(message)} autoHideDuration={6000} onClose={() => setMessage(null)}><Alert severity="error" onClose={() => setMessage(null)}>{message}</Alert></Snackbar>
  </Box>;
}

function EntryMenu({ entry, anchor, onClose, onAction, mutable, editorAvailable }: { entry: FileEntry | null; anchor: HTMLElement | null; onClose: () => void; onAction: (action: string, entry: FileEntry) => void; mutable: boolean; editorAvailable: boolean }) {
  const { t } = useI18n();
  const action = (name: string) => { if (entry) onAction(name, entry); };
  return <Menu anchorEl={anchor} open={Boolean(entry && anchor)} onClose={onClose}>{entry && <Box>
    {entry.kind === 'file' && <MenuItem onClick={() => action('download')}><ListItemIcon><Download fontSize="small" /></ListItemIcon>{t('action.download')}</MenuItem>}
    {entry.kind === 'file' && entry.previewable && <MenuItem onClick={() => action('preview')}>{t('action.preview')}</MenuItem>}
    {entry.kind === 'file' && editorAvailable && <MenuItem onClick={() => action('edit')}><ListItemIcon><Edit fontSize="small" /></ListItemIcon>{t('action.edit')}</MenuItem>}
    <MenuItem onClick={() => action('properties')}>{t('action.properties')}</MenuItem><Divider />
    {mutable && <MenuItem onClick={() => action('rename')}>{t('action.rename')}</MenuItem>}{mutable && <MenuItem onClick={() => action('move')}>{t('action.move')}</MenuItem>}{mutable && <MenuItem onClick={() => action('copy')}>{t('action.copy')}</MenuItem>}
    {mutable && entry.kind === 'directory' && <MenuItem onClick={() => action('archive')}>{t('action.archive')}</MenuItem>}{mutable && entry.isArchive && <MenuItem onClick={() => action('extract')}>{t('action.extract')}</MenuItem>}
    {mutable && entry.kind === 'file' && <MenuItem onClick={() => action('checksum')}>{t('action.checksum')}</MenuItem>}{mutable && <MenuItem onClick={() => action('delete')} sx={{ color: 'error.main' }}>{t('action.delete')}</MenuItem>}
  </Box>}</Menu>;
}

function EntryForm({ state, currentPath, selectedEntries, onClose, onSubmit, onBatchSubmit }: { state: FormState; currentPath: string; selectedEntries: FileEntry[]; onClose: () => void; onSubmit: (endpoint: string, values: Record<string, string>) => void; onBatchSubmit: (endpoint: string, destination?: string) => void }) {
  const { t } = useI18n(); const [value, setValue] = useState(''); const [destination, setDestination] = useState(currentPath); const [picker, setPicker] = useState(false);
  useEffect(() => { if (state) { setValue(state.entry?.name ?? ''); setDestination(currentPath); } }, [state, currentPath]);
  if (!state) return null;
  const title: Record<FormAction, string> = { newdir: t('dialog.newFolder'), newfile: t('dialog.newFile'), rename: t('dialog.rename'), move: t('action.move'), copy: t('action.copy') };
  const isMultiple = !state.entry && ['move', 'copy'].includes(state.action);
  const submit = () => {
    if (state.action === 'newdir') onSubmit('do/newdir', { path: currentPath, dirname: value });
    else if (state.action === 'newfile') onSubmit('do/newfile', { path: currentPath, filename: value });
    else if (state.action === 'rename' && state.entry) onSubmit('do/rename', { path: state.entry.path, name: value });
    else if (isMultiple) onBatchSubmit(`do/batch/${state.action}`, destination);
    else if (state.entry) onSubmit(`do/${state.action}`, { path: state.entry.path, destination, ...(value && value !== state.entry.name ? { name: value } : {}) });
    onClose();
  };
  return <Dialog open onClose={onClose} fullWidth maxWidth="xs"><DialogTitle>{title[state.action]}</DialogTitle><DialogContent><Stack spacing={2} sx={{ pt: 1 }}>
    {!isMultiple && <TextField autoFocus fullWidth required label={state.action === 'newdir' ? t('dialog.folderName') : state.action === 'newfile' ? t('dialog.fileName') : t('dialog.newName')} value={value} onChange={(event) => setValue(event.target.value)} />}
    {(state.action === 'move' || state.action === 'copy') && <TextField fullWidth label={t('dialog.destination')} value={destination || t('workspace.root')} InputProps={{ readOnly: true, endAdornment: <Button onClick={() => setPicker(true)}>{t('action.chooseFolder')}</Button> }} />}
    {isMultiple && <Typography>{t('workspace.selected', { count: selectedEntries.length })}</Typography>}
  </Stack></DialogContent><DialogActions><Button onClick={onClose}>{t('action.cancel')}</Button><Button variant="contained" disabled={(!isMultiple && !value.trim()) || ((state.action === 'move' || state.action === 'copy') && destination === currentPath)} onClick={submit}>{t('action.confirm')}</Button></DialogActions><FolderPicker open={picker} initialPath={destination} onClose={() => setPicker(false)} onChoose={(path) => { setDestination(path); setPicker(false); }} /></Dialog>;
}

function FolderPicker({ open, initialPath, onClose, onChoose }: { open: boolean; initialPath: string; onClose: () => void; onChoose: (path: string) => void }) {
  const { t } = useI18n(); const [path, setPath] = useState(initialPath); const query = useQuery({ queryKey: ['directories', path], queryFn: () => api.getDirectories(path), enabled: open });
  useEffect(() => { if (open) setPath(initialPath); }, [open, initialPath]);
  return <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs"><DialogTitle>{t('dialog.destination')}</DialogTitle><DialogContent><Breadcrumbs sx={{ my: 1 }}><Button onClick={() => setPath('')}>{t('workspace.root')}</Button>{path.split('/').filter(Boolean).map((part, index, all) => <Button key={`${part}-${index}`} onClick={() => setPath(all.slice(0, index + 1).join('/'))}>{part}</Button>)}</Breadcrumbs><List dense>{query.isLoading && <CircularProgress size={24} />}{query.data?.dirs.map((dir) => <ListItemButton key={dir.path} onClick={() => setPath(dir.path)}><ListItemIcon><Folder color="primary" /></ListItemIcon><ListItemText primary={dir.name} /></ListItemButton>)}</List></DialogContent><DialogActions><Button onClick={onClose}>{t('action.cancel')}</Button><Button variant="contained" onClick={() => onChoose(path)}>{t('action.chooseFolder')}</Button></DialogActions></Dialog>;
}

function PropertiesDialog({ entry, properties, isLoading, onClose }: { entry: FileEntry | null; properties?: Properties; isLoading: boolean; onClose: () => void }) {
  const { t } = useI18n(); return <Drawer anchor="right" open={Boolean(entry)} onClose={onClose} PaperProps={{ sx: { width: 'min(100vw, 400px)', p: 3 } }}><Stack spacing={2}><Typography variant="h6">{t('properties.title')}</Typography>{isLoading && <CircularProgress />}{properties && <List disablePadding>{[[t('properties.type'), properties.kind], [t('properties.location'), properties.path], [t('properties.size'), fmtBytes(properties.size)], [t('properties.modified'), fmtDate(new Date(properties.modified * 1000).toISOString())], [t('properties.permissions'), properties.mode], [t('properties.contents'), properties.entry_count?.toString() ?? '—']].map(([label, value]) => <ListItemText key={label} primary={label} secondary={value} sx={{ py: 1 }} />)}{properties.incomplete && <Alert severity="info">{t('properties.incomplete')}</Alert>}</List>}</Stack></Drawer>;
}
