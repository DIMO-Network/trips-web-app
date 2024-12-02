import './polyfill';

import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';
import '@rainbow-me/rainbowkit/styles.css';
import { getDefaultConfig } from '@rainbow-me/rainbowkit';
import { WagmiProvider } from 'wagmi';
import { arbitrum, base, mainnet, optimism, polygon, zora } from 'wagmi/chains';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DimoAuthProvider } from '@dimo-network/login-with-dimo';

const config = getDefaultConfig({
    appName: 'Trips Web App',
    projectId: import.meta.env.DIMO_PROJECT_ID,
    chains: [mainnet, polygon, optimism, arbitrum, base, zora],
});

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <WagmiProvider config={config}>
            <DimoAuthProvider>
                <QueryClientProvider client={queryClient}>
                    <App />
                </QueryClientProvider>
            </DimoAuthProvider>
        </WagmiProvider>
    </React.StrictMode>
);
