/** The text size setting moves the terminal font too: small 12, default 13, large 15. */
export const termFontSize = (size: string | undefined) => (size === 'small' ? 12 : size === 'large' ? 15 : 13)
