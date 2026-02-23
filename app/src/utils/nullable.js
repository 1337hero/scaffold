function nullable(field) {
  if (!field?.Valid) return null;
  return field.String ?? field.Int64 ?? null;
}

export { nullable };
