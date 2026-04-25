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
  'text-\\[9px\\]': 'text-xs',
  'text-\\[10px\\]': 'text-sm',
  'text-\\[11px\\]': 'text-sm',
  'text-xs': 'text-sm',
  'text-sm': 'text-base',
  // 'text-base': 'text-lg',
};

walk(dir).forEach(file => {
  let content = fs.readFileSync(file, 'utf8');
  let newContent = content;
  
  // Custom replace function
  for (const [k, v] of Object.entries(map)) {
    // Look for space, quote or backtick before, and space, quote or backtick after
    const regex = new RegExp(`(?<=[\\s"'\\\`])${k}(?=[\\s"'\\\`])`, 'g');
    newContent = newContent.replace(regex, v);
  }
  
  if (content !== newContent) {
    fs.writeFileSync(file, newContent, 'utf8');
    console.log(`Gentle bumped fonts in ${file}`);
  }
});
