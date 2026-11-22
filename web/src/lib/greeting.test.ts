import { expect, test } from 'vitest';
import { productName } from './greeting';

test('productName', () => {
  expect(productName()).toBe('lookfar');
});
