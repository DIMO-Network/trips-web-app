import {ApiService} from "@services/api-service.ts";

export interface PublicSettings {
    "clientId": `0x${string}`,
    "loginUrl": string
}

export interface PrivateSettings {
    devicesApiUrl: string,
    accountsApiUrl: string,
    paymasterUrl: string,
    rpcUrl: string,
    bundlerUrl: string,
    environment: "prod" | "dev",
    turnkeyOrgId: string,
    turnkeyApiUrl: string,
    turnkeyRpId: string,
}

export interface AccountInfo {
    "subOrganizationId": string,
    "isDeployed": boolean,
    "hasPasskey": boolean,
    "emailVerified": boolean,
    "authenticators": any[] // do we need more details here?
}

const PRIVATE_SETTINGS_KEY = "appPrivateSettings";
const PUBLIC_SETTINGS_KEY = "appPublicSettings";
const ACCOUNT_INFO_KEY = "accountInfo";

export class SettingsService {
    static instance = new SettingsService();

    // TODO: Make those private later
    publicSettings?: PublicSettings;
    privateSettings?: PrivateSettings;
    accountInfo?: AccountInfo;

    private apiService = ApiService.getInstance();

    static getInstance() {
        return SettingsService.instance;
    }

    constructor() {
        this.publicSettings = this.loadPublicSettings();
        this.privateSettings = this.loadPrivateSettings();
        this.accountInfo = this.loadAccountInfo();
    }

    async fetchPrivateSettings() {
        const response = await this.apiService.callApi<PrivateSettings>("GET", "/v1/settings", null, true);

        if (response.success) {
            this.privateSettings = response.data!;
            this.savePrivateSettings();
            return this.privateSettings;
        }

        return null;
    }

    async fetchPublicSettings() {
        const response = await this.apiService.callApi<PublicSettings>("GET", "/v1/public/settings", null, true);

        if (response.success) {
            this.publicSettings = response.data!;
            this.savePublicSettings();
            return this.publicSettings;
        }

        return null;
    }

    async fetchAccountInfo(email: string) {
        const apiUrl = this.privateSettings?.accountsApiUrl;
        const url = `${apiUrl}/api/account/${email}`;
        const response = await this.apiService.callApi<AccountInfo>("GET", url, null, false);

        if (response.success) {
            this.accountInfo = response.data!;
            this.saveAccountInfo();
            return this.privateSettings;
        }

        return null;
    }

    savePublicSettings() {
        localStorage.setItem(PUBLIC_SETTINGS_KEY, JSON.stringify(this.publicSettings));
    }

    savePrivateSettings() {
        localStorage.setItem(PRIVATE_SETTINGS_KEY, JSON.stringify(this.privateSettings));
    }

    saveAccountInfo() {
        localStorage.setItem(ACCOUNT_INFO_KEY, JSON.stringify(this.accountInfo));
    }

    loadPublicSettings(): PublicSettings | undefined {
        const ls = localStorage.getItem(PUBLIC_SETTINGS_KEY);
        return ls ? JSON.parse(ls) : undefined;
    }

    loadPrivateSettings(): PrivateSettings | undefined {
        const ls = localStorage.getItem(PRIVATE_SETTINGS_KEY);
        return ls ? JSON.parse(ls) : undefined;
    }

    loadAccountInfo(): AccountInfo | undefined {
        const ls = localStorage.getItem(ACCOUNT_INFO_KEY);
        return ls ? JSON.parse(ls) : undefined;
    }

    /**
     * gets the wallet address returned from LIWD, which happens to be the ZeroDev org smart contract address
     * @returns {string} 0x Organization smart contract address from ZeroDev/Turnkey. Not be confused with the user's wallet address
     */
    getOrgSmartContractAddress(){
        return localStorage.getItem("walletAddress");
    }
    
}