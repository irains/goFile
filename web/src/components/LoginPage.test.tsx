import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import { SessionProvider } from '../session/SessionProvider';
import { LoginPage } from './LoginPage';

describe('LoginPage', () => {
  it('keeps outlined labels shrunk and stable from the first focused render', () => {
    const { container } = render(<I18nProvider><SessionProvider loginPage><LoginPage /></SessionProvider></I18nProvider>);

    for (const name of ['username', 'password']) {
      const input = container.querySelector<HTMLInputElement>(`input[name="${name}"]`);
      const label = input && container.querySelector(`label[for="${input.id}"]`);
      expect(label).toHaveClass('MuiInputLabel-shrink');
      expect(label).not.toHaveClass('MuiInputLabel-animated');
    }

    expect(container.querySelector('input[name="username"]')).toHaveFocus();
  });
});
