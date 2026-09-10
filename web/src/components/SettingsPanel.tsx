import { SettingsOutlined } from '@mui/icons-material';
import { Box, FormControl, FormControlLabel, FormHelperText, FormLabel, Radio, RadioGroup, Stack, Typography } from '@mui/material';
import { useColorScheme } from '@mui/material/styles';
import { useThemePreferences } from '../appearance/ThemePreferencesProvider';
import { useI18n } from '../i18n';
import { accentIds, accentPalettes, type ThemeMode } from '../theme';
import { SidePanel } from './SidePanel';

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
          <FormLabel component="legend" id="settings-accent">{t('settings.accent')}</FormLabel>
          <RadioGroup
            aria-labelledby="settings-accent"
            value={accentId}
            onChange={(event) => setAccentId(event.target.value as typeof accentId)}
            sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, minmax(0, 1fr))' }, gap: 1, mt: 1 }}
          >
            {accentIds.map((id) => {
              const palette = accentPalettes[id][accentScheme];
              return (
                <FormControlLabel
                  key={id}
                  value={id}
                  control={<Radio />}
                  label={
                    <Stack direction="row" spacing={1} alignItems="center" sx={{ minWidth: 0 }}>
                      <Box aria-hidden sx={{ width: 14, height: 14, flex: '0 0 auto', borderRadius: '50%', bgcolor: palette.main, border: '1px solid', borderColor: palette.dark }} />
                      <Typography variant="bodyStrong" component="span">{t(`settings.accent${id[0].toUpperCase()}${id.slice(1)}`)}</Typography>
                    </Stack>
                  }
                  sx={{ minHeight: 48, m: 0, px: 0.75, border: '1px solid', borderColor: accentId === id ? 'primary.main' : 'divider', borderRadius: 1, bgcolor: accentId === id ? 'action.selected' : 'transparent', '&:hover': { bgcolor: accentId === id ? 'action.selected' : 'action.hover' }, '@media (pointer: coarse)': { minHeight: 48 } }}
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
