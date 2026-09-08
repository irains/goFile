import { Navigate, Outlet, createBrowserRouter } from 'react-router-dom';
import { LoginPage } from './components/LoginPage';
import { Workspace } from './components/Workspace';
import { getRuntime } from './runtime';
import { SessionProvider } from './session/SessionProvider';

function LoginRoute() {
  return <SessionProvider loginPage><LoginPage /></SessionProvider>;
}

function ProtectedShell() {
  return <SessionProvider loginPage={false}><Outlet /></SessionProvider>;
}

function UnknownRoute() {
  return <Navigate replace to="/" />;
}

export function createAppRouter() {
  const { basePath } = getRuntime();
  return createBrowserRouter([
    { path: '/login', element: <LoginRoute /> },
    {
      element: <ProtectedShell />,
      children: [
        { path: '/', element: <Workspace />, children: [
          { index: true, element: null },
          { path: 'd/*', element: null },
          { path: 'edit/*', element: null }
        ] }
      ]
    },
    { path: '*', element: <UnknownRoute /> }
  ], { basename: basePath || undefined });
}
