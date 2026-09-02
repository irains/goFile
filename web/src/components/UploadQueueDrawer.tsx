import { useEffect, useMemo, useRef, useState } from 'react';
import { Add, CancelOutlined, DeleteOutline, Pause, PlayArrow, UploadFile } from '@mui/icons-material';
import { Alert, Button, IconButton, LinearProgress, List, ListItem, ListItemIcon, ListItemText, Stack, Typography } from '@mui/material';
import { getRuntime } from '../runtime';
import { useI18n } from '../i18n';
import { ReliableUploadQueue, type QueueItem } from '../uploads/queue';
import { listStoredUploads, uploadScope } from '../uploads/storage';
import { SidePanel } from './SidePanel';
import { EmptyState } from './EmptyState';

function labelFor(phase: QueueItem['phase'], t: (key: string) => string) {
  return phase === 'reselect' ? t('upload.resumeHint') : t(`upload.${phase === 'failed' ? 'failed' : phase}`);
}

export function UploadQueueDrawer({ open, onClose, destination, username, onAllComplete }: { open: boolean; onClose: () => void; destination: string; username: string; onAllComplete: () => void }) {
  const { t } = useI18n();
  const runtime = getRuntime();
  const fileInput = useRef<HTMLInputElement>(null);
  const [items, setItems] = useState<QueueItem[]>([]);
  const [attachmentID, setAttachmentID] = useState<string | null>(null);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const [hasPersisted, setHasPersisted] = useState<boolean>(false);
  const queue = useMemo(() => new ReliableUploadQueue(uploadScope(window.location.origin, runtime.basePath, username), () => destination), [runtime.basePath, username, destination]);
  useEffect(() => {
    const unsubscribe = queue.subscribe(() => setItems(queue.snapshot()));
    return unsubscribe;
  }, [queue]);
  useEffect(() => { void listStoredUploads(uploadScope(window.location.origin, runtime.basePath, username)).then((saved) => { setHasPersisted(saved.length > 0); queue.restore(saved); }); }, [queue, runtime.basePath, username]);
  useEffect(() => { if (items.length > 0 && items.every((item) => ['completed', 'cancelled'].includes(item.phase))) onAllComplete(); }, [items, onAllComplete]);
  const openFilePicker = (id?: string) => {
    setAttachmentID(id ?? null);
    setAttachmentError(null);
    fileInput.current?.click();
  };
  const handleFiles = (files: FileList | null) => {
    if (!files?.length) return;
    if (attachmentID && files.length === 1) {
      const id = attachmentID;
      setAttachmentID(null);
      void queue.attachFile(id, files[0]).then((attached) => { if (!attached) setAttachmentError(t('upload.sourceChanged')); });
      return;
    }
    setAttachmentID(null);
    queue.add([...files]);
  };
  return (
    <SidePanel open={open} onClose={onClose} icon={<UploadFile />} title={t('upload.title')}>
      <Stack spacing={1.5} sx={{ height: '100%' }}>
        <input ref={fileInput} type="file" hidden multiple={!attachmentID} onChange={(event) => { handleFiles(event.target.files); event.currentTarget.value = ''; }} />
        <Button variant="contained" startIcon={<Add />} onClick={() => openFilePicker()}>{t('upload.addFiles')}</Button>
        <Typography variant="caption" color="text.secondary" onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); setAttachmentID(null); handleFiles(event.dataTransfer.files); }}>
          {t('workspace.dropFiles')}
        </Typography>
        {attachmentError && <Alert severity="warning">{attachmentError}</Alert>}
        {!items.length && !hasPersisted && (
          <EmptyState
            icon={<UploadFile />}
            title={t('upload.addFiles')}
            caption={t('workspace.dropFiles')}
            action={<Button variant="contained" startIcon={<Add />} onClick={() => openFilePicker()}>{t('upload.addFiles')}</Button>}
          />
        )}
        {!items.length && hasPersisted && <Alert severity="info">{t('upload.resumeHint')}</Alert>}
        <List sx={{ overflow: 'auto', mx: -1 }}>
          {items.map((item) => <UploadRow item={item} key={item.id} queue={queue} t={t} onAttach={() => openFilePicker(item.id)} />)}
        </List>
      </Stack>
    </SidePanel>
  );
}

function UploadRow({ item, queue, t, onAttach }: { item: QueueItem; queue: ReliableUploadQueue; t: (key: string) => string; onAttach: () => void }) {
  const resumeAction = item.phase === 'uploading' || item.phase === 'hashing' ? <IconButton aria-label={t('action.pause')} onClick={() => queue.pause(item.id)}><Pause /></IconButton>
    : ['paused', 'failed'].includes(item.phase) && item.file ? <IconButton aria-label={t('action.resume')} onClick={() => queue.resume(item.id)}><PlayArrow /></IconButton>
    : ['paused', 'failed', 'reselect'].includes(item.phase) ? <Button size="small" onClick={onAttach}>{t('upload.resumeHint')}</Button> : null;
  const terminal = ['completed', 'cancelled'].includes(item.phase);
  return (
    <ListItem
      alignItems="flex-start"
      secondaryAction={
        <Stack direction="row" spacing={0.5}>
          {resumeAction}
          {!terminal && <IconButton aria-label={t('action.cancelUpload')} onClick={() => void queue.cancel(item.id)}><CancelOutlined /></IconButton>}
          <IconButton aria-label={t('action.remove')} onClick={() => void queue.remove(item.id)}><DeleteOutline /></IconButton>
        </Stack>
      }
      sx={{ px: 1, py: 1.25 }}
    >
      <ListItemIcon sx={{ minWidth: 38 }}>
        <UploadFile color={item.phase === 'failed' ? 'error' : item.phase === 'completed' ? 'success' : 'action'} />
      </ListItemIcon>
      <ListItemText
        primary={<Typography noWrap>{item.name}</Typography>}
        secondary={
          <Stack spacing={0.5}>
            <Typography variant="caption" color={item.phase === 'failed' ? 'error.main' : 'text.secondary'}>{item.error ?? labelFor(item.phase, t)}</Typography>
            <LinearProgress variant="determinate" value={Math.min(100, item.progress * 100)} />
          </Stack>
        }
      />
    </ListItem>
  );
}