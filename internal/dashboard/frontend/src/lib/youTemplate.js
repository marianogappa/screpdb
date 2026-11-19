// youTemplate rewrites an event sentence's template so the viewer reads as "you": "{target}'s" becomes "your",
// a sentence that opens with them becomes "You" with its verb agreeing ("You attack", "You are"), and anywhere
// else they are "you". Only English changes its verbs; other languages just put their word for "you" in.

const IRREGULAR = { is: 'are', does: 'do', has: 'have', was: 'were' };

export const secondPerson = (verb) => {
  const w = verb.toLowerCase();
  if (IRREGULAR[w]) return IRREGULAR[w];
  if (/[^aeiou]ies$/.test(w)) return `${w.slice(0, -3)}y`;
  if (/(sh|ch|ss|x|z)es$/.test(w)) return w.slice(0, -2);
  if (/[^s]s$/.test(w)) return w.slice(0, -1);
  return w;
};

const isVerb = (word) => Boolean(IRREGULAR[word.toLowerCase()]) || /[^s]s$/.test(word);

const capitalize = (text) => text.charAt(0).toUpperCase() + text.slice(1);

// agreeVerb fixes the first verb after a subject, within its clause.
const agreeVerb = (rest) => {
  const clause = rest.search(/[;,.]/);
  const end = clause < 0 ? rest.length : clause;
  const words = rest.slice(0, end).replace(/\{\w+\}/g, ' ').match(/[A-Za-z]+/g) || [];
  const verb = words.find(isVerb);
  if (!verb) return rest;
  const at = rest.search(new RegExp(`\\b${verb}\\b`));
  return rest.slice(0, at) + secondPerson(verb) + rest.slice(at + verb.length);
};

export const youTemplate = (template, roles, words, english) => {
  if (!template || roles.length === 0) return template;
  let out = template;
  for (const role of roles) {
    const ph = `{${role}}`;
    if (english) {
      out = out.replace(new RegExp(`(^|; )\\{${role}\\}('s)?([^]*)$`), (whole, lead, possessive, rest) => {
        if (possessive) return `${lead}${capitalize(words.possessive)}${rest}`;
        const subject = lead ? words.object : capitalize(words.subject);
        return `${lead}${subject}${agreeVerb(rest)}`;
      });
      out = out.split(`${ph}'s`).join(words.possessive);
      if (role === 'actor') out = out.replace(/\btheir\b/g, words.possessive);
      if (role === 'target') out = out.replace(/\bthey have\b/g, `${words.object} have`);
    }
    out = out.split(ph).join(words.object);
  }
  return out;
};
