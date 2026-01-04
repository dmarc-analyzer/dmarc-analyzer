import antfu from '@antfu/eslint-config'

export default antfu({
  ignores: [
    'src/services/openapi/**',
  ],
}).append({
  files: [
    'tsconfig.node.json',
  ],
  rules: {
    'jsonc/sort-keys': 'off',
  },
})
