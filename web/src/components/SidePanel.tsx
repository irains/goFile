import type { ReactNode } from 'react';
import { Box, Drawer, IconButton, Stack, Typography, type SxProps, type Theme } from '@mui/material';
import { Close } from '@mui/icons-material';
import { useI18n } from '../i18n';

type PanelHeaderProps = {
  icon: ReactNode;
  title: string;
  onClose: () => void;
  trailing?: ReactNode;
};

export function PanelHeader({ icon, title, onClose, trailing }: PanelHeaderProps) {
  const { t } = useI18n();
  return (
    <Box
      component="header"
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        px: 2.5,
        py: 2,
        borderBottom: '1px solid',
        borderColor: 'divider',
        bgcolor: 'background.paper'
      }}
    >
      <Box sx={{ display: 'inline-flex', color: 'primary.main' }}>{icon}</Box>
      <Typography variant="title" component="h2" sx={{ flex: 1, minWidth: 0 }}>
        {title}
      </Typography>
      {trailing}
      <IconButton aria-label={t('action.close')} onClick={onClose}>
        <Close />
      </IconButton>
    </Box>
  );
}

type SidePanelProps = {
  open: boolean;
  onClose: () => void;
  icon: ReactNode;
  title: string;
  trailing?: ReactNode;
  children: ReactNode;
  width?: SxProps<Theme>;
};

export function SidePanel({ open, onClose, icon, title, trailing, children, width }: SidePanelProps) {
  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      PaperProps={{ sx: { width: { xs: '100%', sm: 400 }, maxWidth: '100%', display: 'flex', ...(width as object) } }}
    >
      <Stack sx={{ minHeight: 0, flex: 1 }}>
        <PanelHeader icon={icon} title={title} onClose={onClose} trailing={trailing} />
        <Box sx={{ p: 2.5, overflow: 'auto', flex: 1 }}>{children}</Box>
      </Stack>
    </Drawer>
  );
}
