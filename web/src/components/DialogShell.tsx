import type { ReactNode } from 'react';
import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack } from '@mui/material';
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
  hideActions = false
}: DialogShellProps) {
  const { t } = useI18n();
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth={maxWidth}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          {children}
        </Stack>
      </DialogContent>
      {!hideActions && (
        <DialogActions sx={{ px: 3, pb: 2 }}>
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
