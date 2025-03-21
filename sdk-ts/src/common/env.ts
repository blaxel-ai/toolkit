import fs from 'fs';
import toml from 'toml';

const secretEnv: any = {};
let configEnv: any = {};

try {
  const configFile = fs.readFileSync('blaxel.toml', 'utf8');
  const configInfos = toml.parse(configFile);
  configEnv = configInfos.env;
/* eslint-disable */
} catch (error) {
}

try {
  const secretFile = fs.readFileSync('.env', 'utf8');
  secretFile.split('\n').forEach((line) => {
    if(line.startsWith('#')) {
      return;
    }
    const [key, value] = line.split('=');
    secretEnv[key] = value;
  });
} catch (error) {
}

const env = new Proxy({}, {
  get: (target, prop: string) => {
    if (secretEnv[prop]) {
      return secretEnv[prop];
    }
    if (configEnv[prop]) {
      return configEnv[prop];
    }
    return process.env[prop];
  }
});

export { env };
