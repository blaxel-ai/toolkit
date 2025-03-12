export class Credentials {
    async authenticate() {
        await new Promise(resolve => setTimeout(resolve, 1000));
    }

    get authorization() {
        return ''
    }
}
