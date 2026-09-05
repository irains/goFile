import { useEffect, useState } from 'react';
import { ChevronRight, Folder, KeyboardArrowUp } from '@mui/icons-material';
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Skeleton,
  Stack,
  Tooltip,
  Typography
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { ApiError, api } from '../api/client';
import { useI18n } from '../i18n';
import { fontFamilyMono, radii, semantic, surface } from '../tokens';

export type FolderDestinationPickerProps = {
  value: string;
  onChange: (path: string) => void;
};

const pathLabel = (path: string, root: string) => path || root;

export function FolderDestinationPicker({ value, onChange }: FolderDestinationPickerProps) {
  const { t } = useI18n();
  const [path, setPath] = useState(value);
  const query = useQuery({ queryKey: ['directories', path], queryFn: () => api.getDirectories(path) });
  const displayPath = pathLabel(path, t('workspace.root'));
  const segments = path.split('/').filter(Boolean);

  useEffect(() => { setPath(value); }, [value]);

  const navigate = (next: string) => {
    setPath(next);
    onChange(next);
  };
  const goUp = () => navigate(segments.slice(0, -1).join('/'));

  return (
    <Stack spacing={1.5} sx={{ minHeight: 0, flex: 1 }}>
      <Box
        aria-label={t('folderPicker.selected', { path: displayPath })}
        sx={{
          ...surface,
          display: 'flex',
          alignItems: 'center',
          gap: 1.25,
          px: 1.5,
          py: 1.25,
          minWidth: 0,
          bgcolor: 'action.selected'
        }}
      >
        <Folder sx={{ color: semantic.folder, flex: '0 0 auto' }} />
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="caption" color="text.secondary">{t('dialog.destination')}</Typography>
          <Typography
            component="div"
            variant="bodyStrong"
            title={displayPath}
            sx={{ fontFamily: fontFamilyMono, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
          >
            {displayPath}
          </Typography>
        </Box>
      </Box>

      <Stack direction="row" spacing={0.5} alignItems="center" sx={{ minWidth: 0 }}>
        <Tooltip title={t('workspace.upOneLevel')}>
          <span>
            <IconButton aria-label={t('workspace.upOneLevel')} onClick={goUp} disabled={segments.length === 0} size="small">
              <KeyboardArrowUp fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Box sx={{ minWidth: 0, flex: 1, overflowX: 'auto', whiteSpace: 'nowrap', '& .MuiBreadcrumbs-ol': { flexWrap: 'nowrap' } }}>
          <Breadcrumbs aria-label={t('folderPicker.breadcrumbs', { path: displayPath })}>
            <Button size="small" onClick={() => navigate('')} sx={{ flexShrink: 0 }}>{t('workspace.root')}</Button>
            {segments.map((segment, index) => {
              const segmentPath = segments.slice(0, index + 1).join('/');
              const isCurrent = index === segments.length - 1;
              return isCurrent ? (
                <Typography key={segmentPath} variant="caption" color="text.secondary" sx={{ flexShrink: 0 }} aria-current="page">{segment}</Typography>
              ) : (
                <Button key={segmentPath} size="small" onClick={() => navigate(segmentPath)} sx={{ flexShrink: 0 }}>{segment}</Button>
              );
            })}
          </Breadcrumbs>
        </Box>
      </Stack>

      <Box
        aria-label={t('folderPicker.listLabel', { path: displayPath })}
        sx={{ ...surface, minHeight: 176, maxHeight: { xs: 'none', sm: 320 }, overflow: 'auto', flex: 1, borderRadius: `${radii.sm}px` }}
      >
        {query.isLoading && (
          <Stack role="status" aria-live="polite" spacing={1} sx={{ p: 1.25 }}>
            <Typography variant="caption" color="text.secondary">{t('folderPicker.loading')}</Typography>
            {[0, 1, 2].map((index) => <Skeleton key={index} variant="rounded" height={44} />)}
          </Stack>
        )}
        {query.isError && (
          <Stack spacing={1.25} sx={{ p: 2 }}>
            <Alert severity="error">{query.error instanceof ApiError ? t(`error.${query.error.code}`) : t('folderPicker.loadFailed')}</Alert>
            <Box><Button size="small" onClick={() => void query.refetch()}>{t('action.retry')}</Button></Box>
          </Stack>
        )}
        {query.data?.dirs.length === 0 && (
          <Stack alignItems="center" justifyContent="center" spacing={0.75} sx={{ minHeight: 176, px: 2, textAlign: 'center' }}>
            <Folder sx={{ color: semantic.folderMuted, fontSize: 32 }} />
            <Typography variant="bodyStrong">{t('folderPicker.empty')}</Typography>
            <Typography variant="caption" color="text.secondary">{t('folderPicker.currentFolder')}</Typography>
          </Stack>
        )}
        {query.data?.dirs && query.data.dirs.length > 0 && (
          <List disablePadding aria-label={t('folderPicker.listLabel', { path: displayPath })}>
            {query.data.dirs.map((directory) => (
              <ListItemButton key={directory.path} onClick={() => navigate(directory.path)} sx={{ minHeight: 44, px: 1.5 }}>
                <ListItemIcon sx={{ minWidth: 32, color: semantic.folder }}><Folder fontSize="small" /></ListItemIcon>
                <ListItemText
                  primary={directory.name}
                  primaryTypographyProps={{ noWrap: true, title: directory.name, variant: 'bodyStrong' }}
                  sx={{ minWidth: 0, mr: 1 }}
                />
                <ChevronRight fontSize="small" color="action" aria-hidden="true" />
              </ListItemButton>
            ))}
          </List>
        )}
      </Box>
    </Stack>
  );
}
