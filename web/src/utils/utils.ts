/**
 * checks if running under localhost or our local dev hostname
 * @returns {boolean}
 */
export function isLocalhost() {
    return window.location.hostname === "localhost" ||
        window.location.hostname === "localdev.dimo.org" ||
        window.location.hostname === "";
}

/**
 * Converts an ECDSA signature (r, s, v) into a full Ethereum hex signature.
 *
 * Ethereum uses a 65-byte signature format: `r (32 bytes) + s (32 bytes) + v (1 byte)`.
 * This function ensures `v` is correctly formatted (27 or 28) before concatenating.
 *
 * @param {Object} signResult - The signature result object.
 * @param {string} signResult.r - The 32-byte hex string representing the `r` value.
 * @param {string} signResult.s - The 32-byte hex string representing the `s` value.
 * @param {string} signResult.v - The recovery ID as a hex string (typically `"00"` or `"01"`).
 * @returns {`0x${string}`} The full Ethereum signature as a 0x-prefixed hex string.
 */
// @ts-ignore FIXME: something is wrong with types, the Signature type from `viem` uses `v` as bigint, not string
export function formatEthereumSignature(signResult) {
    const { r, s, v } = signResult;
    const vHex = (parseInt(v, 16) + 27).toString(16).padStart(2, '0');
    return `0x${r}${s}${vHex}`;
}