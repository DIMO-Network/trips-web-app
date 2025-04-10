import {html, LitElement, css} from 'lit'
import {SettingsService} from "@services/settings-service";

export class LoginElement extends LitElement {
    static properties = {
        clientId: {type: String},
        token: {type: String},
        alertText: {type: String},
        loginUrl: {type: String},
    }
    private loginBaseUrl: string;
    private loginUrl: string;
    private settings: SettingsService;
    private clientId: string;

    constructor() {
        super();
        this.loginBaseUrl = '';
        this.loginUrl = '';
        this.settings = SettingsService.getInstance();
        this.clientId = '';
    }

    async connectedCallback() {
        super.connectedCallback();

        const settings = await this.settings.fetchPublicSettings();
        this.clientId = settings?.clientId || ""
        this.loginBaseUrl = settings?.loginUrl || ""

        if (this.clientId.length === 42) {
            this.setupLoginUrl();
        }
    }

    static styles = css`
    .login-with-dimo-link {
        color: #fff;
        background-color: #0070f3;
        padding: 10px 20px;
        border-radius: 5px;
        text-decoration: none;
    }
    `

    render() {
        return html`
            <div class="grid place-items-center" ?hidden=${this.loginUrl === ""}>
                <a id="loginLink" href="${this.loginUrl}" class="login-with-dimo-link">Login with DIMO!</a>
            </div>
            <div class="grid place-items-center" ?hidden=${this.loginUrl !== ""}>
                <h3>It appears there is no ClientID configured</h3>
                <p>If you don't have a Client ID please go to the <a href="https://console.dimo.org">DIMO Developer Console</a></p>
            </div>
        `;
    }

    setupLoginUrl() {
        let redirectUrl = "";
        // Check if the hostname is "localhost" or "127.0.0.1"
        redirectUrl = location.origin + "/login.html";
        this.loginUrl = `${this.loginBaseUrl}?clientId=${this.clientId}&redirectUri=${redirectUrl}&entryState=EMAIL_INPUT`;
    }
}
window.customElements.define('login-element', LoginElement);