import './App.css';
import { useEffect } from 'react';
import {
  initializeDimoSDK,
  LoginWithDimo,
  ShareVehiclesWithDimo,
  useDimoAuthState,
  DimoAuthProvider,
} from '@dimo-network/login-with-dimo';
import logo from './assets/whole_logo.png';

interface AuthData {
  token: string;
}

initializeDimoSDK({
  clientId: import.meta.env.DIMO_CLIENT_ID,
  redirectUri: import.meta.env.DIMO_REDIRECT_URI,
  apiKey: import.meta.env.DIMO_API_KEY,
  environment: 'production',
});

const permissionTemplateId = import.meta.env.DIMO_PERMISSION_TEMPLATE_ID;

function App() {
  const { isAuthenticated, getValidJWT } = useDimoAuthState();

  useEffect(() => {
    if (isAuthenticated) {
      const jwt = getValidJWT();
      console.log('User authenticated. JWT:', jwt);
    }
  }, [isAuthenticated]);

  const handleSuccess = (authData: AuthData) => {
    console.log('Login Success:', authData.token);

    // Send the JWT to the backend to establish the session
    fetch('/login-jwt', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({ jwt: authData.token }),
    })
        .then((response) => {
          if (response.ok) {
            console.log('Session established, redirecting to /vehicles/me');
          } else {
            console.error('Failed to establish session, response:', response);
          }
        })
        .catch((error) => {
          console.error('Error sending JWT to backend:', error);
        });

    window.location.href = `${import.meta.env.DIMO_API_BASEURL}/vehicles/me`;
  };

  return (
      <DimoAuthProvider>
        <div className="app-container">
          <div className="logo-container">
            <img src={logo} alt="Logo" className="logo" />
          </div>
          <div className="login-button-container">
            {isAuthenticated ? (
                <ShareVehiclesWithDimo
                    mode="popup"
                    permissionTemplateId={permissionTemplateId}
                    onSuccess={handleSuccess}
                    onError={(error: Error) => {
                      console.error('Error sharing vehicles:', error);
                    }}
                />
            ) : (
                <LoginWithDimo
                    mode="popup"
                    permissionTemplateId={permissionTemplateId}
                    onSuccess={handleSuccess}
                    onError={(error: Error) => {
                      console.error('Authentication error:', error);
                    }}
                />
            )}
          </div>
        </div>
      </DimoAuthProvider>
  );
}

export default App;