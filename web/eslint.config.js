import boundaries from 'eslint-plugin-boundaries';

export default [
  {
    plugins: {
      boundaries,
    },
    settings: {
      'boundaries/elements': [
        { type: 'app', pattern: 'src/app/*' },
        { type: 'pages', pattern: 'src/pages/*' },
        { type: 'features', pattern: 'src/features/*' },
        { type: 'components', pattern: 'src/components/*' },
        { type: 'api', pattern: 'src/api/*' },
        { type: 'lib', pattern: 'src/lib/*' },
      ],
    },
    rules: {
      'boundaries/element-types': [
        'error',
        {
          default: 'disallow',
          rules: [
            { from: 'app', allow: ['pages', 'features', 'components', 'api', 'lib'] },
            { from: 'pages', allow: ['features', 'components', 'lib'] },
            { from: 'features', allow: ['features', 'components', 'lib', 'api'] },
            { from: 'components', allow: ['lib'] },
            { from: 'api', allow: ['lib'] },
          ],
        },
      ],
    },
  },
];
