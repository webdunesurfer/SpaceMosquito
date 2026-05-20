const path = require('path');
const CopyPlugin = require('copy-webpack-plugin');

module.exports = (env, argv) => ({
  entry: {
    background: './background.ts',
    'content/content': './content/content.ts',
    'popup/popup': './popup/popup.ts',
  },
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name].js',
    clean: true,
  },
  resolve: {
    extensions: ['.ts', '.js'],
  },
  module: {
    rules: [
      {
        test: /\.ts$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
    ],
  },
  plugins: [
    new CopyPlugin({
      patterns: [
        { from: 'manifest.json', to: 'manifest.json' },
        { from: 'popup/popup.html', to: 'popup/popup.html' },
        { from: 'popup/popup.css', to: 'popup/popup.css' },
        { from: 'assets', to: 'assets' },
      ],
    }),
  ],
  devtool: argv.mode === 'development' ? 'source-map' : false,
});
