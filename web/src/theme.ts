export const themeModeStorageKey = 'fileharbor-mode';
export const themeSchemeStorageKey = 'fileharbor-color-scheme';
export const themeSchemeAttribute = 'data-mui-color-scheme';
export const themeAccentStorageKey = 'fileharbor-accent';
export const themeAccentAttribute = 'data-fileharbor-accent';

export type ThemeMode = 'light' | 'dark' | 'system';
export type ColorScheme = 'light' | 'dark';
export type AccentId =
  | 'forest'
  | 'harbor'
  | 'slate'
  | 'orchid'
  | 'cedar'
  | 'fjord'
  | 'juniper'
  | 'ember'
  | 'dune'
  | 'saffron'
  | 'mulberry'
  | 'graphite';

type PaletteColor = {
  main: string;
  light: string;
  dark: string;
  contrastText: string;
};

type PaletteText = {
  primary: string;
  secondary: string;
  disabled: string;
};

type PaletteAction = {
  hover: string;
  selected: string;
  disabled: string;
  disabledBackground: string;
};

type PaletteTexture = {
  dot: string;
  wash: string;
  glow: string;
};

export type FullPaletteScheme = {
  primary: PaletteColor;
  secondary: PaletteColor;
  canvas: string;
  paper: string;
  appBar: string;
  overlay: string;
  tableHeader: string;
  text: PaletteText;
  divider: string;
  action: PaletteAction;
  focus: string;
  shadow: string;
  texture: PaletteTexture;
};

export type AccentPalette = Record<ColorScheme, FullPaletteScheme>;

export const accentIds: readonly AccentId[] = [
  'forest',
  'harbor',
  'slate',
  'orchid',
  'cedar',
  'fjord',
  'juniper',
  'ember',
  'dune',
  'saffron',
  'mulberry',
  'graphite'
];
export const defaultAccentId: AccentId = 'forest';

// Every palette intentionally owns the surrounding surfaces as well as its controls.
// The values are direct sRGB colors so MUI and first-paint CSS can agree exactly.
export const accentPalettes: Record<AccentId, AccentPalette> = {
  forest: {
    light: {
      primary: { main: '#3C6A4D', light: '#70967A', dark: '#2D533B', contrastText: '#FBF9F2' },
      secondary: { main: '#286470', light: '#5C9299', dark: '#1D4C55', contrastText: '#FBF9F2' },
      canvas: '#F3F0E7', paper: '#FBF9F2', appBar: '#EDE9DE', overlay: '#F7F5EE', tableHeader: '#F4F1E8',
      text: { primary: '#242820', secondary: '#596052', disabled: '#7A8074' }, divider: '#D9D5C6',
      action: { hover: '#E7EEE6', selected: '#DCE9DB', disabled: '#7A8074', disabledBackground: '#E7E6DE' },
      focus: '#2D533B', shadow: '#2D533B', texture: { dot: '67 84 54', wash: '89 111 74', glow: '37 99 109' }
    },
    dark: {
      primary: { main: '#9EC6A0', light: '#C4DEC4', dark: '#729D77', contrastText: '#171D18' },
      secondary: { main: '#84C5CC', light: '#B4E0E2', dark: '#55939B', contrastText: '#171D18' },
      canvas: '#171D18', paper: '#222A22', appBar: '#121812', overlay: '#273027', tableHeader: '#1D251E',
      text: { primary: '#F0F1E7', secondary: '#C1C7B9', disabled: '#899286' }, divider: '#3B463C',
      action: { hover: '#293329', selected: '#354635', disabled: '#899286', disabledBackground: '#2A302A' },
      focus: '#9EC6A0', shadow: '#050806', texture: { dot: '219 231 207', wash: '128 159 113', glow: '100 172 180' }
    }
  },
  harbor: {
    light: {
      primary: { main: '#236676', light: '#5A92A0', dark: '#174D5A', contrastText: '#F7FBFA' },
      secondary: { main: '#3F7079', light: '#77A0A6', dark: '#29555D', contrastText: '#F7FBFA' },
      canvas: '#EDF3F2', paper: '#F7FBFA', appBar: '#E3EFED', overlay: '#F2F8F7', tableHeader: '#EAF3F1',
      text: { primary: '#203035', secondary: '#52676A', disabled: '#748185' }, divider: '#CCDDDB',
      action: { hover: '#E1EFED', selected: '#D0E6E3', disabled: '#748185', disabledBackground: '#E0E8E6' },
      focus: '#174D5A', shadow: '#174D5A', texture: { dot: '45 90 101', wash: '55 116 132', glow: '81 145 157' }
    },
    dark: {
      primary: { main: '#8EC8D3', light: '#B8E1E7', dark: '#5A9EAA', contrastText: '#142022' },
      secondary: { main: '#A1D2D5', light: '#C3E5E6', dark: '#6EA6AA', contrastText: '#142022' },
      canvas: '#142022', paper: '#1D2C2E', appBar: '#101B1D', overlay: '#263638', tableHeader: '#192729',
      text: { primary: '#EDF5F4', secondary: '#C1D0CE', disabled: '#80908F' }, divider: '#344A4B',
      action: { hover: '#293D3E', selected: '#345153', disabled: '#80908F', disabledBackground: '#273334' },
      focus: '#8EC8D3', shadow: '#05090A', texture: { dot: '212 235 238', wash: '105 168 181', glow: '154 205 208' }
    }
  },
  slate: {
    light: {
      primary: { main: '#4C617C', light: '#7B91AD', dark: '#36485F', contrastText: '#FAFBFD' },
      secondary: { main: '#3E6B77', light: '#7397A0', dark: '#294F59', contrastText: '#FAFBFD' },
      canvas: '#EFF2F6', paper: '#FAFBFD', appBar: '#E5EAF0', overlay: '#F6F8FB', tableHeader: '#E9EDF3',
      text: { primary: '#29313D', secondary: '#5C6775', disabled: '#7A8491' }, divider: '#D2D9E2',
      action: { hover: '#E7ECF3', selected: '#D9E2EF', disabled: '#7A8491', disabledBackground: '#E4E8ED' },
      focus: '#36485F', shadow: '#36485F', texture: { dot: '70 84 106', wash: '83 107 139', glow: '94 137 151' }
    },
    dark: {
      primary: { main: '#B5C6E1', light: '#D7E1F1', dark: '#8398B7', contrastText: '#191D26' },
      secondary: { main: '#9FC9D1', light: '#C3E0E4', dark: '#6D9FA9', contrastText: '#191D26' },
      canvas: '#191D26', paper: '#242A35', appBar: '#151922', overlay: '#2D3440', tableHeader: '#202631',
      text: { primary: '#F0F3F8', secondary: '#C4CBD6', disabled: '#8993A0' }, divider: '#404957',
      action: { hover: '#303846', selected: '#3A4759', disabled: '#8993A0', disabledBackground: '#2B303B' },
      focus: '#B5C6E1', shadow: '#06070A', texture: { dot: '224 231 244', wash: '141 164 200', glow: '148 188 200' }
    }
  },
  orchid: {
    light: {
      primary: { main: '#74557F', light: '#A17DAA', dark: '#553B60', contrastText: '#FCF9FC' },
      secondary: { main: '#5D657C', light: '#8C94AA', dark: '#434A60', contrastText: '#FCF9FC' },
      canvas: '#F4EFF4', paper: '#FCF9FC', appBar: '#ECE4EC', overlay: '#F8F3F8', tableHeader: '#F0EAF0',
      text: { primary: '#342D37', secondary: '#695E6B', disabled: '#867A88' }, divider: '#DED3DE',
      action: { hover: '#EEE5EE', selected: '#E3D5E5', disabled: '#867A88', disabledBackground: '#EAE3EA' },
      focus: '#553B60', shadow: '#553B60', texture: { dot: '104 78 111', wash: '127 91 137', glow: '111 111 151' }
    },
    dark: {
      primary: { main: '#D2B7DE', light: '#E7D5ED', dark: '#A48AB2', contrastText: '#211923' },
      secondary: { main: '#B9C1DE', light: '#D7DCF0', dark: '#8993B4', contrastText: '#211923' },
      canvas: '#211923', paper: '#2D2230', appBar: '#1B141D', overlay: '#382A3A', tableHeader: '#281E2B',
      text: { primary: '#F5EFF6', secondary: '#D1C4D2', disabled: '#968B98' }, divider: '#514153',
      action: { hover: '#3B2E3D', selected: '#4C3A50', disabled: '#968B98', disabledBackground: '#342936' },
      focus: '#D2B7DE', shadow: '#09060A', texture: { dot: '239 226 243', wash: '178 139 190', glow: '177 183 222' }
    }
  },
  cedar: {
    light: {
      primary: { main: '#8A572E', light: '#B78359', dark: '#69401E', contrastText: '#FCF9F4' },
      secondary: { main: '#516B61', light: '#81988E', dark: '#385148', contrastText: '#FCF9F4' },
      canvas: '#F5F0E9', paper: '#FCF9F4', appBar: '#EDE4D8', overlay: '#F9F4ED', tableHeader: '#F1E9DE',
      text: { primary: '#35291E', secondary: '#6B5B4B', disabled: '#897868' }, divider: '#DFD0BF',
      action: { hover: '#F0E5D8', selected: '#EAD7C3', disabled: '#897868', disabledBackground: '#ECE2D6' },
      focus: '#69401E', shadow: '#69401E', texture: { dot: '118 77 44', wash: '151 96 55', glow: '98 130 115' }
    },
    dark: {
      primary: { main: '#E7BF84', light: '#F3DBAF', dark: '#BB8D52', contrastText: '#221B16' },
      secondary: { main: '#A8CCBC', light: '#C5E0D3', dark: '#759D8D', contrastText: '#221B16' },
      canvas: '#221B16', paper: '#30261E', appBar: '#1B1511', overlay: '#3A2E24', tableHeader: '#2A211A',
      text: { primary: '#F7F0E6', secondary: '#D5C5B4', disabled: '#9C8D7D' }, divider: '#564536',
      action: { hover: '#3C3025', selected: '#513C2B', disabled: '#9C8D7D', disabledBackground: '#34291F' },
      focus: '#E7BF84', shadow: '#0A0705', texture: { dot: '244 226 194', wash: '200 151 85', glow: '160 202 188' }
    }
  },
  fjord: {
    light: {
      primary: { main: '#1F5C7A', light: '#5687A2', dark: '#16465F', contrastText: '#F7FBFD' },
      secondary: { main: '#47736F', light: '#78A09C', dark: '#2E5653', contrastText: '#F7FBFD' },
      canvas: '#EDF3F7', paper: '#F8FBFD', appBar: '#E1ECF2', overlay: '#F3F8FB', tableHeader: '#E7F0F5',
      text: { primary: '#21313C', secondary: '#536975', disabled: '#75838D' }, divider: '#C9D9E2',
      action: { hover: '#E1EEF4', selected: '#D0E4EF', disabled: '#75838D', disabledBackground: '#E1E8EC' },
      focus: '#16465F', shadow: '#16465F', texture: { dot: '42 91 120', wash: '76 133 160', glow: '79 132 128' }
    },
    dark: {
      primary: { main: '#94C9E3', light: '#C2E2F0', dark: '#639CB8', contrastText: '#14212A' },
      secondary: { main: '#A6D0C6', light: '#C7E5DD', dark: '#75A79B', contrastText: '#14212A' },
      canvas: '#14212A', paper: '#1E2C36', appBar: '#101B22', overlay: '#283944', tableHeader: '#1A2731',
      text: { primary: '#EDF5F8', secondary: '#C3D2D9', disabled: '#83949B' }, divider: '#364C59',
      action: { hover: '#2A3D49', selected: '#365365', disabled: '#83949B', disabledBackground: '#26343E' },
      focus: '#94C9E3', shadow: '#04080A', texture: { dot: '214 237 247', wash: '101 160 192', glow: '166 208 198' }
    }
  },
  juniper: {
    light: {
      primary: { main: '#216759', light: '#5D9482', dark: '#164D43', contrastText: '#F7FBF8' },
      secondary: { main: '#526B48', light: '#81987A', dark: '#394F31', contrastText: '#F7FBF8' },
      canvas: '#EFF5F1', paper: '#F8FCF9', appBar: '#E4EFE8', overlay: '#F3F8F4', tableHeader: '#E9F2EB',
      text: { primary: '#24342D', secondary: '#576B61', disabled: '#78877F' }, divider: '#CCDDD3',
      action: { hover: '#E4F0E7', selected: '#D2E7DA', disabled: '#78877F', disabledBackground: '#E2EAE5' },
      focus: '#164D43', shadow: '#164D43', texture: { dot: '43 103 84', wash: '84 139 119', glow: '102 128 83' }
    },
    dark: {
      primary: { main: '#9ED7C1', light: '#C4E8D6', dark: '#6FA990', contrastText: '#15221C' },
      secondary: { main: '#B8D4A5', light: '#D5E8C9', dark: '#87AA75', contrastText: '#15221C' },
      canvas: '#15221C', paper: '#1F3028', appBar: '#101B16', overlay: '#293C32', tableHeader: '#1A2A22',
      text: { primary: '#EDF6F0', secondary: '#C3D4C8', disabled: '#85968B' }, divider: '#385247',
      action: { hover: '#2A4035', selected: '#365646', disabled: '#85968B', disabledBackground: '#26372E' },
      focus: '#9ED7C1', shadow: '#040806', texture: { dot: '215 239 225', wash: '113 169 144', glow: '184 212 165' }
    }
  },
  ember: {
    light: {
      primary: { main: '#994C3C', light: '#C77A66', dark: '#733629', contrastText: '#FDF9F7' },
      secondary: { main: '#6D6650', light: '#999079', dark: '#504A38', contrastText: '#FDF9F7' },
      canvas: '#F7F0ED', paper: '#FDF9F7', appBar: '#F0E2DC', overlay: '#FBF4F1', tableHeader: '#F3E9E4',
      text: { primary: '#382923', secondary: '#6C5B53', disabled: '#8B786F' }, divider: '#E2CEC6',
      action: { hover: '#F2E4DE', selected: '#EBCFC5', disabled: '#8B786F', disabledBackground: '#EDE0DA' },
      focus: '#733629', shadow: '#733629', texture: { dot: '139 66 52', wash: '181 84 65', glow: '148 104 67' }
    },
    dark: {
      primary: { main: '#F0AE9B', light: '#F7CCBE', dark: '#C97969', contrastText: '#251917' },
      secondary: { main: '#D4C38B', light: '#E8DAB3', dark: '#A69661', contrastText: '#251917' },
      canvas: '#251917', paper: '#33231F', appBar: '#1E1412', overlay: '#402B26', tableHeader: '#2D1F1B',
      text: { primary: '#F9F0EC', secondary: '#D7C4BD', disabled: '#9D8982' }, divider: '#5B4039',
      action: { hover: '#402C28', selected: '#563933', disabled: '#9D8982', disabledBackground: '#382723' },
      focus: '#F0AE9B', shadow: '#0B0605', texture: { dot: '248 220 211', wash: '204 121 105', glow: '212 195 139' }
    }
  },
  dune: {
    light: {
      primary: { main: '#7A6242', light: '#A58B67', dark: '#5C482F', contrastText: '#FCFAF5' },
      secondary: { main: '#5D7269', light: '#8BA097', dark: '#41554D', contrastText: '#FCFAF5' },
      canvas: '#F5F1E8', paper: '#FCFAF5', appBar: '#ECE5D7', overlay: '#F9F6EE', tableHeader: '#F0EADF',
      text: { primary: '#342D22', secondary: '#675E50', disabled: '#847A6B' }, divider: '#DDD4C3',
      action: { hover: '#EEE7D9', selected: '#E4D8C5', disabled: '#847A6B', disabledBackground: '#E9E3D8' },
      focus: '#5C482F', shadow: '#5C482F', texture: { dot: '112 92 62', wash: '151 127 88', glow: '105 133 122' }
    },
    dark: {
      primary: { main: '#DBC49C', light: '#ECDDBD', dark: '#B49A70', contrastText: '#211C15' },
      secondary: { main: '#B6D1C5', light: '#D0E4DB', dark: '#86A99B', contrastText: '#211C15' },
      canvas: '#211C15', paper: '#2D281F', appBar: '#19150F', overlay: '#383126', tableHeader: '#282219',
      text: { primary: '#F7F2E8', secondary: '#D4CABB', disabled: '#988D7D' }, divider: '#50483A',
      action: { hover: '#393227', selected: '#4C4231', disabled: '#988D7D', disabledBackground: '#332C22' },
      focus: '#DBC49C', shadow: '#090705', texture: { dot: '239 230 205', wash: '180 154 112', glow: '182 209 197' }
    }
  },
  saffron: {
    light: {
      primary: { main: '#8C6116', light: '#B98A38', dark: '#69470E', contrastText: '#FCFAF4' },
      secondary: { main: '#526C5A', light: '#829A88', dark: '#394F41', contrastText: '#FCFAF4' },
      canvas: '#F8F2E5', paper: '#FCFAF4', appBar: '#F0E5CA', overlay: '#FBF6EA', tableHeader: '#F4EAD5',
      text: { primary: '#372D1B', secondary: '#6A5E43', disabled: '#887A5E' }, divider: '#E3D2AE',
      action: { hover: '#F3E8D0', selected: '#EEDCB4', disabled: '#887A5E', disabledBackground: '#EDE5D5' },
      focus: '#69470E', shadow: '#69470E', texture: { dot: '134 92 19', wash: '185 138 56', glow: '104 134 105' }
    },
    dark: {
      primary: { main: '#E9C56E', light: '#F3DB9C', dark: '#BD9137', contrastText: '#241B0E' },
      secondary: { main: '#AFD0B6', light: '#CEE4D1', dark: '#7EAA8A', contrastText: '#241B0E' },
      canvas: '#241B0E', paper: '#322715', appBar: '#1C150A', overlay: '#3D301A', tableHeader: '#2C2111',
      text: { primary: '#FAF2DF', secondary: '#D8C8A9', disabled: '#9E8D69' }, divider: '#59482A',
      action: { hover: '#40321A', selected: '#574523', disabled: '#9E8D69', disabledBackground: '#382B16' },
      focus: '#E9C56E', shadow: '#0B0702', texture: { dot: '247 229 183', wash: '200 151 55', glow: '180 211 184' }
    }
  },
  mulberry: {
    light: {
      primary: { main: '#6F3E63', light: '#9C6B91', dark: '#522C49', contrastText: '#FCF9FB' },
      secondary: { main: '#526475', light: '#8293A4', dark: '#394959', contrastText: '#FCF9FB' },
      canvas: '#F5EFF4', paper: '#FCF9FB', appBar: '#ECE2EA', overlay: '#F8F3F7', tableHeader: '#F0E8EF',
      text: { primary: '#352936', secondary: '#695A69', disabled: '#867787' }, divider: '#DFD0DE',
      action: { hover: '#F0E5EF', selected: '#E5D3E2', disabled: '#867787', disabledBackground: '#EBE2EA' },
      focus: '#522C49', shadow: '#522C49', texture: { dot: '105 59 93', wash: '149 88 132', glow: '96 123 145' }
    },
    dark: {
      primary: { main: '#E0AED0', light: '#EFD0E5', dark: '#B57DA5', contrastText: '#251723' },
      secondary: { main: '#B5C9DF', light: '#D5E0EF', dark: '#839DB9', contrastText: '#251723' },
      canvas: '#251723', paper: '#332034', appBar: '#1D121C', overlay: '#402943', tableHeader: '#2D1C2E',
      text: { primary: '#F8EFF7', secondary: '#D4C3D2', disabled: '#998A98' }, divider: '#584257',
      action: { hover: '#422B43', selected: '#583958', disabled: '#998A98', disabledBackground: '#392439' },
      focus: '#E0AED0', shadow: '#0A0509', texture: { dot: '244 223 240', wash: '188 126 174', glow: '181 201 223' }
    }
  },
  graphite: {
    light: {
      primary: { main: '#4F5962', light: '#7C8992', dark: '#384149', contrastText: '#FAFBFB' },
      secondary: { main: '#466D73', light: '#78999D', dark: '#315156', contrastText: '#FAFBFB' },
      canvas: '#F0F2F2', paper: '#FAFBFB', appBar: '#E5E9E9', overlay: '#F6F8F8', tableHeader: '#E9ECEC',
      text: { primary: '#2A3133', secondary: '#5D686B', disabled: '#7B8587' }, divider: '#D1D8D8',
      action: { hover: '#E7EBEB', selected: '#D9E1E1', disabled: '#7B8587', disabledBackground: '#E3E7E7' },
      focus: '#384149', shadow: '#384149', texture: { dot: '75 86 91', wash: '112 126 130', glow: '89 138 145' }
    },
    dark: {
      primary: { main: '#C2CBD0', light: '#DCE2E5', dark: '#909DA5', contrastText: '#1C2224' },
      secondary: { main: '#A7CED0', light: '#C8E2E2', dark: '#759FA1', contrastText: '#1C2224' },
      canvas: '#1C2224', paper: '#273033', appBar: '#171C1E', overlay: '#313C3F', tableHeader: '#222A2D',
      text: { primary: '#F0F4F4', secondary: '#C5CDCD', disabled: '#8C9898' }, divider: '#424D50',
      action: { hover: '#354143', selected: '#435256', disabled: '#8C9898', disabledBackground: '#2D3638' },
      focus: '#C2CBD0', shadow: '#060809', texture: { dot: '225 232 232', wash: '143 159 162', glow: '167 208 208' }
    }
  }
};

const MEDIA_DARK = '(prefers-color-scheme: dark)';
const SCHEME_LIGHT: ColorScheme = 'light';
const SCHEME_DARK: ColorScheme = 'dark';

export function isThemeMode(value: unknown): value is ThemeMode {
  return value === 'light' || value === 'dark' || value === 'system';
}

export function isAccentId(value: unknown): value is AccentId {
  return typeof value === 'string' && accentIds.includes(value as AccentId);
}

export function readStoredThemeMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(themeModeStorageKey);
    return isThemeMode(raw) ? raw : 'system';
  } catch {
    return 'system';
  }
}

export function readStoredAccentId(): AccentId {
  try {
    const raw = localStorage.getItem(themeAccentStorageKey);
    return isAccentId(raw) ? raw : defaultAccentId;
  } catch {
    return defaultAccentId;
  }
}

export function persistAccentId(accentId: AccentId) {
  try {
    localStorage.setItem(themeAccentStorageKey, accentId);
  } catch {
    // Browser privacy settings may block storage. The current-tab state remains usable.
  }
}

export function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
  return window.matchMedia(MEDIA_DARK).matches;
}

// Resolves the effective `data-mui-color-scheme` value for a stored intent.
// `system` falls through to the OS preference.
export function resolveColorScheme(intent: ThemeMode | undefined): ColorScheme {
  if (intent === 'light') return SCHEME_LIGHT;
  if (intent === 'dark') return SCHEME_DARK;
  return systemPrefersDark() ? SCHEME_DARK : SCHEME_LIGHT;
}

// Ace editor theme depends on the *resolved* color scheme (not the stored intent).
export function aceThemeForMode(mode: ColorScheme | undefined) {
  return mode === 'light' ? 'github_light_default' : 'github_dark';
}
