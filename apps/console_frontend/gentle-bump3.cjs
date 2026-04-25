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
  'text-[9px]': 'text-xs',
  'text-[10px]': 'text-sm',
  'text-[11px]': 'text-sm',
  'text-xs': 'text-sm',
  'text-sm': 'text-base',
};

walk(dir).forEach(file => {
  let content = fs.readFileSync(file, 'utf8');
  let newContent = content;
  
  // Create a regex that matches any of the keys
  const keys = Object.keys(map).map(k => k.replace(/\[/g, '\\[').replace(/\]/g, '\\]'));
  const regex = new RegExp(`(?<=[\\s"'\\\`])(${keys.join('|')})(?=[\\s"'\\\`])`, 'g');
  
  newContent = newContent.replace(regex, (match) => {
    return map[match];
  });
  
  if (content !== newContent) {
    fs.writeFileSync(file, newContent, 'utf8');
    console.log(`Gentle bumped fonts in ${file}`);
  }
});
