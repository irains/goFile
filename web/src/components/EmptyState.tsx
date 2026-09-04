import type { ReactNode } from 'react';
import { Box, Stack, Typography } from '@mui/material';

type EmptyStateProps = {
  icon: ReactNode;
  title: string;
  caption?: string;
  action?: ReactNode;
};

export function EmptyState({ icon, title, caption, action }: EmptyStateProps) {
  return (
    <Stack alignItems="center" justifyContent="center" spacing={1.5} sx={{ py: 6, px: 3, textAlign: 'center' }}>
      <Box sx={{ color: 'primary.light', display: 'inline-flex', '& svg': { fontSize: 52 } }}>{icon}</Box>
      <Typography variant="title" component="p">{title}</Typography>
      {caption && (
        <Typography variant="caption" color="text.secondary" sx={{ maxWidth: 360 }}>
          {caption}
        </Typography>
      )}
      {action && <Box sx={{ pt: 1 }}>{action}</Box>}
    </Stack>
  );
}
