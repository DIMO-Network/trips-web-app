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
                  mode="redirect"
                  onSuccess={(authData: string) => {
                    console.log("Authentication successful, JWT:", authData);
                    localStorage.setItem("jwt", authData);
                    setStatus("authenticated");
                  }}
                  onError={(error: Error) => {
                    console.error("Authentication error:", error.message);
                  }}
                  clientId={"0xf5ada890DA2E5582E38DF4648F9dAeE00e691199"}
                  redirectUri={"https://trips-sandbox.drivedimo.com/vehicles/me"}
                  environment={"production"}
                  permissionTemplateId={"1"}

              />


            </div>
          </>
        </RainbowKitProvider>
      </RainbowKitAuthenticationProvider>
  );

}

export default App
