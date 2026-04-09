/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.html"],
  theme: {
    extend: {
      colors: {
        'drk': {
          700: '#b91c1c',
          800: '#991b1b',
        }
      }
    },
  },
  plugins: [],
}
