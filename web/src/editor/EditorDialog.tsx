import 'ace-builds/css/ace.css';
import 'ace-builds/css/theme/github_dark.css';
import 'ace-builds/css/theme/github_light_default.css';
import { useEffect, useRef, useState } from 'react';
import type { ComponentType } from 'react';
import { useColorScheme } from '@mui/material/styles';
import { Alert, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Skeleton, Stack, Tooltip, Typography } from '@mui/material';
import type { FileEntry } from '../api/client';
import { ApiError, api } from '../api/client';
import { useI18n } from '../i18n';
import { resolveColorScheme, aceThemeForMode, type ThemeMode } from '../theme';
import { DialogShell } from '../components/DialogShell';

type AceComponent = ComponentType<any>;

type AceDom = {
  useStrictCSP: (enabled: boolean) => void;
};

type AceModule = {
  require: (module: string) => AceDom;
};

function aceModule(value: unknown): AceModule {
  const module = value as { default?: AceModule } & Partial<AceModule>;
  const result = module.default ?? module;
  if (typeof result.require !== 'function') throw new Error('Ace module is unavailable');
  return { require: result.require };
}

function installAceStaticStyles(dom: AceDom) {
  dom.useStrictCSP(true);
}

function modeFor(name: string) {
  const extension = name.split('.').pop()?.toLowerCase() ?? 'text';
  return ({ js: 'javascript', ts: 'typescript', tsx: 'tsx', jsx: 'jsx', json: 'json', md: 'markdown', yml: 'yaml', yaml: 'yaml', sh: 'sh', bash: 'sh', go: 'golang', py: 'python', html: 'html', css: 'css', xml: 'xml', sql: 'sql', toml: 'toml', ini: 'ini' } as Record<string, string>)[extension] ?? 'text';
}

export function EditorDialog({ entry, writable, onClose }: { entry: FileEntry; writable: boolean; onClose: () => void }) {
  const { mode } = useColorScheme();
  const { t } = useI18n();
  const aceTheme = aceThemeForMode(resolveColorScheme((mode ?? 'system') as ThemeMode));
  const [Ace, setAce] = useState<AceComponent | null>(null);
  const [value, setValue] = useState('');
  const [original, setOriginal] = useState('');
  const [version, setVersion] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [conflicted, setConflicted] = useState(false);
  const [saving, setSaving] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const dirty = value !== original;
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;

  const load = async (replaceDraft = true) => {
    setError(null);
    try {
      const response = await api.getEditorContent(entry.path);
      if (replaceDraft) {
        setValue(response.editor.content ?? '');
        setOriginal(response.editor.content ?? '');
      }
      setVersion(response.editor.version);
      setConflicted(false);
    } catch (loadError) {
      setError(loadError instanceof ApiError ? t(`error.${loadError.code}`) : t('error.generic'));
    }
  };

  useEffect(() => {
    let mounted = true;
    void import('ace-builds/src-noconflict/ace').then((ace) => {
      installAceStaticStyles(aceModule(ace).require('ace/lib/dom'));
      return Promise.all([
        import('react-ace'),
        import('ace-builds/src-noconflict/theme-github_dark'),
        import('ace-builds/src-noconflict/theme-github_light_default'),
        import('ace-builds/src-noconflict/mode-javascript'), import('ace-builds/src-noconflict/mode-typescript'), import('ace-builds/src-noconflict/mode-tsx'),
        import('ace-builds/src-noconflict/mode-json'), import('ace-builds/src-noconflict/mode-markdown'), import('ace-builds/src-noconflict/mode-yaml'),
        import('ace-builds/src-noconflict/mode-sh'), import('ace-builds/src-noconflict/mode-golang'), import('ace-builds/src-noconflict/mode-python'),
        import('ace-builds/src-noconflict/mode-html'), import('ace-builds/src-noconflict/mode-css'), import('ace-builds/src-noconflict/mode-xml'),
        import('ace-builds/src-noconflict/mode-sql'), import('ace-builds/src-noconflict/mode-toml')
      ]);
    }).then(([module]) => {
      if (mounted) setAce(() => module.default as unknown as AceComponent);
    }).catch(() => { if (mounted) setError(t('error.generic')); });
    void load();
    return () => { mounted = false; };
  // The entry defines the document lifetime. Translation changes do not reload an edit buffer.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entry.path]);

  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => {
      if (!dirtyRef.current) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, []);

  const save = async (): Promise<boolean> => {
    if (!writable || !version) return false;
    setSaving(true); setError(null);
    try {
      const response = await api.saveEditorContent(entry.path, value, version);
      setOriginal(value);
      setVersion(response.editor.version);
      setConflicted(false);
      return true;
    } catch (saveError) {
      if (saveError instanceof ApiError && saveError.code === 'source_changed') setConflicted(true);
      setError(saveError instanceof ApiError ? t(`error.${saveError.code}`) : t('error.generic'));
      return false;
    } finally { setSaving(false); }
  };
  useEffect(() => {
    const shortcut = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
        event.preventDefault();
        if (dirty && writable && !saving) void save();
      }
    };
    window.addEventListener('keydown', shortcut);
    return () => window.removeEventListener('keydown', shortcut);
  // save is intentionally recreated with the current editor buffer; including
  // it here would rebind this lightweight listener on every keystroke.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dirty, saving, writable, version, value]);
  const close = () => { if (dirty) setDiscarding(true); else onClose(); };
  const saveAndClose = async () => {
    if (await save()) {
      setDiscarding(false);
      onClose();
    }
  };
  const language = modeFor(entry.name);
  return (
    <Dialog open onClose={close} fullScreen>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1.5, borderBottom: '1px solid', borderColor: 'divider' }}>
        <Stack sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="bodyStrong" noWrap>{entry.path}</Typography>
          <Typography variant="caption" color="text.secondary">{language}</Typography>
        </Stack>
        <Tooltip title={t('editor.languageHint')}><Chip size="small" label={language} /></Tooltip>
        {!writable && <Chip size="small" variant="outlined" label={t('editor.readOnly')} />}
        {dirty && <Chip size="small" color="warning" label={t('editor.unsaved')} />}
      </DialogTitle>
      <DialogContent dividers sx={{ p: 0, display: 'flex', flexDirection: 'column' }}>
        {error && <Alert severity={conflicted ? 'warning' : 'error'} action={conflicted ? <Button color="inherit" size="small" onClick={() => void load(true)}>{t('editor.reload')}</Button> : undefined}>{error}</Alert>}
        {Ace ? <Ace mode={language} theme={aceTheme} name={`editor-${entry.path}`} width="100%" height="100%" value={value} readOnly={!writable} onChange={setValue} onLoad={(editor: { focus: () => void }) => editor.focus()} setOptions={{ useWorker: false, showPrintMargin: false, fontSize: 14 }} /> : <Stack role="status" aria-live="polite" alignItems="center" justifyContent="center" sx={{ flex: 1, p: 4 }}><Skeleton variant="rectangular" width="100%" height={400} animation={false} /><Typography variant="caption" color="text.secondary" sx={{ mt: 2 }}>{t('editor.loading')}</Typography></Stack>}
      </DialogContent>
      <DialogActions sx={{ borderTop: '1px solid', borderColor: 'divider' }}>
        <Button onClick={close}>{t('action.close')}</Button>
        {writable && <Button variant="contained" disabled={saving || !dirty || !version} onClick={() => void save()}>{t('editor.save')}</Button>}
      </DialogActions>
      <DialogShell open={discarding} onClose={() => setDiscarding(false)} title={t('dialog.confirmDiscard')} hideActions>
        <Typography color="text.secondary">{t('editor.discardWarning')}</Typography>
        <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{ pt: 1 }}>
          <Button onClick={() => setDiscarding(false)}>{t('action.cancel')}</Button>
          <Button color="error" onClick={() => { setDiscarding(false); onClose(); }}>{t('dialog.discard')}</Button>
          {writable && <Button variant="contained" disabled={saving} onClick={() => void saveAndClose()}>{t('editor.save')}</Button>}
        </Stack>
      </DialogShell>
    </Dialog>
  );
}