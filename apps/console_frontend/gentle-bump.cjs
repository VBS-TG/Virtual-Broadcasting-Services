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
  'text-\\[9px\\]': 'text-\\[11px\\]',
  'text-\\[10px\\]': 'text-xs',
  'text-\\[11px\\]': 'text-sm',
  // 'text-xs': 'text-sm', // Don't touch text-xs for now, let it be 12px
};

walk(dir).forEach(file => {
  let content = fs.readFileSync(file, 'utf8');
  let newContent = content;
  for (const [k, v] of Object.entries(map)) {
    const regex = new RegExp(`(?<!-)\\b${k}\\b`, 'g');
    newContent = newContent.replace(regex, v);
  }
  if (content !== newContent) {
    fs.writeFileSync(file, newContent, 'utf8');
    console.log(`Gentle bumped fonts in ${file}`);
  }
});
