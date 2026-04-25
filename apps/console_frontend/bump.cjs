const fs = require('fs');
const path = require('path');

const dir = 'src';

function walk(d) {
  let files = [];
  fs.readdirSync(d).forEach(f => {
    let p = path.join(d, f);
    if (fs.statSync(p).isDirectory()) files.push(...walk(p));
    else if (p.endsWith('.tsx')) files.push(p);
  });
  return files;
}

const map = {
  'text-\\[9px\\]': 'text-sm',
  'text-\\[10px\\]': 'text-base',
  'text-\\[11px\\]': 'text-lg',
  'text-xs': 'text-xl',
  'text-sm': 'text-2xl',
  'text-base': 'text-3xl',
  'text-lg': 'text-4xl',
  'text-xl': 'text-5xl',
  'text-2xl': 'text-6xl'
};

walk(dir).forEach(file => {
  let content = fs.readFileSync(file, 'utf8');
  let newContent = content;
  // Replace from smallest to largest to avoid double replacements
  // using regex with word boundaries
  for (const [k, v] of Object.entries(map)) {
    const regex = new RegExp(`(?<!-)\\b${k}\\b`, 'g'); // negative lookbehind so md:text-xs matches, but not something-text-xs
    newContent = newContent.replace(regex, v);
  }
  if (content !== newContent) {
    fs.writeFileSync(file, newContent, 'utf8');
    console.log(`Updated fonts in ${file}`);
  }
});
