const apiEndpoint = '';
const userPoolId = 'us-east-1_wrzv1kQeA';
const userPoolClientId = '2ubhlcsrljahkqhh2hlurocmpm';

const apiPath: string = 'http://127.0.0.1:6767/api';

export const commonEnvironment = {
  apiBaseUrl: `${apiEndpoint}${apiPath}`,
  Cognito: {
    userPoolId,
    userPoolClientId,
  },
};
