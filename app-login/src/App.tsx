import { AuthenticationStatus, ConnectButton } from '@rainbow-me/rainbowkit';
import './App.css'
import { useAccount } from 'wagmi';
import { useEffect, useState } from 'react';
import { RainbowKitProvider, createAuthenticationAdapter,
  RainbowKitAuthenticationProvider } from '@rainbow-me/rainbowkit';


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
                  <div>
                    <ConnectButton />
                  </div>
                  <div>
                    Connect your wallet to see your DIMO vehicles!
                  </div>
                </>
            </RainbowKitProvider>
          </RainbowKitAuthenticationProvider>
  )
}

export default App
