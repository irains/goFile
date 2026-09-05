import type { ReactNode } from 'react';
import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack, type SxProps, type Theme } from '@mui/material';
import { useI18n } from '../i18n';

type ConfirmTone = 'default' | 'destructive';

type DialogShellProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  cancelLabel?: string;
  confirmLabel?: string;
  onConfirm?: () => void;
  confirmDisabled?: boolean;
  confirmTone?: ConfirmTone;
  maxWidth?: 'xs' | 'sm' | 'md';
  fullScreen?: boolean;
  contentSx?: SxProps<Theme>;
  stackSx?: SxProps<Theme>;
  actionsSx?: SxProps<Theme>;
  hideActions?: boolean;
};

export function DialogShell({
  open,
  onClose,
  title,
  children,
  cancelLabel,
  confirmLabel,
  onConfirm,
  confirmDisabled = false,
  confirmTone = 'default',
  maxWidth = 'xs',
  fullScreen = false,
  contentSx,
  stackSx,
  actionsSx,
  hideActions = false
}: DialogShellProps) {
  const { t } = useI18n();
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth={maxWidth} fullScreen={fullScreen} PaperProps={{ sx: fullScreen ? { display: 'flex', flexDirection: 'column' } : undefined }}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent sx={contentSx}>
        <Stack spacing={2} sx={{ pt: 1, ...stackSx }}>
          {children}
        </Stack>
      </DialogContent>
      {!hideActions && (
        <DialogActions sx={{ px: 3, pb: 2, ...actionsSx }}>
          <Button onClick={onClose}>{cancelLabel ?? t('action.cancel')}</Button>
          {onConfirm && (
            <Button
              variant="contained"
              color={confirmTone === 'destructive' ? 'error' : 'primary'}
              disabled={confirmDisabled}
              onClick={onConfirm}
            >
              {confirmLabel ?? t('action.confirm')}
            </Button>
          )}
        </DialogActions>
      )}
    </Dialog>
  );
}
