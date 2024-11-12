import { AuthenticationStatus, ConnectButton } from '@rainbow-me/rainbowkit';
import './App.css'
import { useAccount } from 'wagmi';
import { useEffect, useState } from 'react';
import { RainbowKitProvider, createAuthenticationAdapter,
  RainbowKitAuthenticationProvider } from '@rainbow-me/rainbowkit';
import logo from './assets/whole_logo.png';
import { LoginWithDimo } from 'dimo-login-button-sdk';


class DIMODexMessage {
  state?: string;
  challenge?: string;
  constructor(param: Partial<DIMODexMessage>) {
    this.state = param.state;
    this.challenge = param.challenge;
  }
};

function App() {
  const [status, setStatus] = useState<AuthenticationStatus>("unauthenticated");
  const account = useAccount();

  const authenticationAdapter = createAuthenticationAdapter({
    getNonce: async () => {
      const address = account?.address;
        
      const response = await fetch(`${import.meta.env.DIMO_API_BASEURL}/auth/web3/generate_challenge`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            address,
        }),
      });
  
      if (!response.ok) {
        throw new Error('Failed to fetch nonce');
      }  

      const challengeResponse:string = await response.text();
      return challengeResponse; 
    },
  
    createMessage: ({nonce}) => {

      const message : { state:string, challenge: string } = JSON.parse(nonce);

      return new DIMODexMessage({
        state: message.state,
        challenge: message.challenge,
      });
    },
  
    getMessageBody: ({message}) => {
      return message.challenge as string;
    },
  
    verify: async ({ message, signature }) => {
      const verifyRes = await fetch(`${import.meta.env.DIMO_API_BASEURL}/auth/web3/submit_challenge`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ state: message.state, signature }),
      });
  
      if (!verifyRes.ok) {
        throw new Error('Failed to verify signature');
      }
  
      setStatus('authenticated');
  
      return verifyRes.ok;
    },
  
    signOut: async () => {
      await fetch('/api/logout');
    },
  
  });

  useEffect(() => {
    if (status === 'authenticated') {
      // redirect to handlebars
      window.location.href = `${import.meta.env.DIMO_API_BASEURL}/vehicles/me`;
    }
  }
  , [status]);

  const clientId = import.meta.env.DIMO_CLIENT_ID;
  const redirectUri = import.meta.env.DIMO_REDIRECT_URI;
  const environment = import.meta.env.DIMO_ENVIRONMENT;
  const permissionTemplateId = import.meta.env.DIMO_PERMISSION_TEMPLATE_ID;
  const mode = import.meta.env.DIMO_MODE;

  console.log("DIMO_API_BASEURL:", import.meta.env.DIMO_API_BASEURL);
  console.log("CLIENT_ID:", import.meta.env.DIMO_CLIENT_ID);

  return (
      <RainbowKitAuthenticationProvider adapter={authenticationAdapter} status={status}>
        <RainbowKitProvider>
          <>
            <div className="logo-container">
              <img src={logo} alt="Logo" className="logo" />
            </div>
            <div className="connect-button-container">
              <ConnectButton />
            </div>
            <div className="connect-button-container">
              <p><a href="/login-jwt">Login with JWT</a></p>
            </div>
            <div className="connect-button-container">
              <LoginWithDimo
                  mode={mode}
                  onSuccess={(authData: string) => {
                    console.log("Authentication successful, JWT:", authData);
                    localStorage.setItem("jwt", authData);
                    setStatus("authenticated");
                  }}
                  onError={(error: Error) => {
                    console.error("Authentication error:", error.message);
                  }}
                  clientId={clientId}
                  redirectUri={redirectUri}
                  environment={environment}
                  permissionTemplateId={permissionTemplateId}

              />


            </div>
          </>
        </RainbowKitProvider>
      </RainbowKitAuthenticationProvider>
  );

}

export default App
