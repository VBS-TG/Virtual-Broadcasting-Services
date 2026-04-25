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

const reverseMap = {
  'text-6xl': 'text-2xl',
  'text-5xl': 'text-xl',
  'text-4xl': 'text-lg',
  'text-3xl': 'text-base',
  'text-2xl': 'text-sm',
  'text-xl': 'text-xs',
  'text-lg': 'text-\\[11px\\]',
  'text-base': 'text-\\[10px\\]',
  'text-sm': 'text-\\[9px\\]'
};

walk(dir).forEach(file => {
  let content = fs.readFileSync(file, 'utf8');
  let newContent = content;
  // Replace from largest to smallest to avoid double replacements
  for (const [k, v] of Object.entries(reverseMap)) {
    const regex = new RegExp(`(?<!-)\\b${k}\\b`, 'g');
    newContent = newContent.replace(regex, v.replace('\\', ''));
  }
  if (content !== newContent) {
    fs.writeFileSync(file, newContent, 'utf8');
    console.log(`Reverted fonts in ${file}`);
  }
});
