import { SettingsOutlined } from '@mui/icons-material';
import { Box, FormControl, FormControlLabel, FormHelperText, FormLabel, Radio, RadioGroup, Stack, Typography } from '@mui/material';
import { useColorScheme } from '@mui/material/styles';
import { useThemePreferences } from '../appearance/ThemePreferencesProvider';
import { useI18n } from '../i18n';
import { accentIds, accentPalettes, type AccentId, type ThemeMode } from '../theme';
import { SidePanel } from './SidePanel';

function paletteLabelKey(id: AccentId) {
  return `settings.palette${id[0].toUpperCase()}${id.slice(1)}` as const;
}

function PalettePreview({ palette }: { palette: (typeof accentPalettes)[AccentId]['light'] }) {
  return (
    <Box
      aria-hidden
      sx={{
        position: 'relative',
        width: 66,
        height: 48,
        flex: '0 0 auto',
        overflow: 'hidden',
        border: '1px solid',
        borderColor: palette.divider,
        borderRadius: 0.75,
        bgcolor: palette.canvas,
        backgroundImage: `radial-gradient(circle at 1px 1px, rgb(${palette.texture.dot} / 14%) 0 0.5px, transparent 0.7px)`,
        backgroundSize: '7px 7px'
      }}
    >
      <Box sx={{ height: 7, bgcolor: palette.appBar, borderBottom: '1px solid', borderColor: palette.divider }} />
      <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 14px', gap: 0.5, p: 0.5 }}>
        <Box sx={{ height: 30, p: 0.5, border: '1px solid', borderColor: palette.divider, bgcolor: palette.paper }}>
          <Box sx={{ height: 3, width: '68%', bgcolor: palette.text.primary, opacity: 0.78 }} />
          <Box sx={{ height: 3, mt: 0.5, width: '88%', bgcolor: palette.divider }} />
          <Box sx={{ height: 4, mt: 0.75, bgcolor: palette.action.selected, border: '1px solid', borderColor: palette.primary.light }} />
        </Box>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          <Box sx={{ height: 10, borderRadius: 0.5, bgcolor: palette.primary.main }} />
          <Box sx={{ flex: 1, border: '1px solid', borderColor: palette.divider, bgcolor: palette.overlay }} />
        </Box>
      </Box>
    </Box>
  );
}

export function SettingsPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode, systemMode } = useColorScheme();
  const { accentId, setAccentId } = useThemePreferences();
  const selectedMode: ThemeMode = mode === 'light' || mode === 'dark' || mode === 'system' ? mode : 'system';
  const accentScheme = selectedMode === 'light' || (selectedMode === 'system' && systemMode === 'light') ? 'light' : 'dark';

  return (
    <SidePanel open={open} onClose={onClose} icon={<SettingsOutlined />} title={t('settings.title')}>
      <Stack spacing={3}>
        <FormControl component="fieldset">
          <FormLabel component="legend" id="settings-appearance">{t('settings.appearance')}</FormLabel>
          <RadioGroup aria-labelledby="settings-appearance" value={selectedMode} onChange={(event) => setMode(event.target.value as ThemeMode)} sx={{ mt: 0.75 }}>
            <FormControlLabel value="light" control={<Radio />} label={t('settings.modeLight')} />
            <FormControlLabel value="dark" control={<Radio />} label={t('settings.modeDark')} />
            <FormControlLabel value="system" control={<Radio />} label={t('settings.modeSystem')} />
          </RadioGroup>
          <FormHelperText>{t('settings.systemHint')}</FormHelperText>
        </FormControl>

        <FormControl component="fieldset">
          <FormLabel component="legend" id="settings-palette">{t('settings.palette')}</FormLabel>
          <FormHelperText sx={{ mt: 0.5 }}>{t('settings.paletteHint')}</FormHelperText>
          <RadioGroup
            aria-labelledby="settings-palette"
            value={accentId}
            onChange={(event) => setAccentId(event.target.value as AccentId)}
            sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1, mt: 1 }}
          >
            {accentIds.map((id) => {
              const palette = accentPalettes[id][accentScheme];
              const checked = accentId === id;
              return (
                <FormControlLabel
                  key={id}
                  value={id}
                  control={<Radio size="small" />}
                  label={
                    <Stack direction="row" spacing={0.75} alignItems="center" sx={{ minWidth: 0, flex: 1 }}>
                      <PalettePreview palette={palette} />
                      <Typography variant="bodyStrong" component="span" sx={{ lineHeight: 1.25, overflowWrap: 'anywhere' }}>
                        {t(paletteLabelKey(id))}
                      </Typography>
                    </Stack>
                  }
                  sx={{
                    minHeight: 92,
                    m: 0,
                    px: 0.75,
                    py: 0.5,
                    alignItems: 'center',
                    border: '1px solid',
                    borderColor: checked ? 'primary.main' : 'divider',
                    borderRadius: 1,
                    bgcolor: checked ? 'action.selected' : 'transparent',
                    '& .MuiRadio-root': { p: 0.5, mr: 0.25 },
                    '& .MuiFormControlLabel-label': { minWidth: 0, display: 'flex', flex: 1 },
                    '&:hover': { bgcolor: checked ? 'action.selected' : 'action.hover' },
                    '&:has(.MuiRadio-root.Mui-focusVisible)': { outline: '2px solid', outlineColor: 'primary.main', outlineOffset: 2 },
                    '@media (pointer: coarse)': { minHeight: 92 }
                  }}
                />
              );
            })}
          </RadioGroup>
        </FormControl>

        <FormControl component="fieldset">
          <FormLabel component="legend" id="settings-language">{t('settings.language')}</FormLabel>
          <RadioGroup aria-labelledby="settings-language" value={locale} onChange={(event) => setLocale(event.target.value as 'en' | 'zh')} sx={{ mt: 0.75 }}>
            <FormControlLabel value="en" control={<Radio />} label={t('settings.languageEnglish')} />
            <FormControlLabel value="zh" control={<Radio />} label={t('settings.languageChinese')} />
          </RadioGroup>
        </FormControl>
      </Stack>
    </SidePanel>
  );
}
