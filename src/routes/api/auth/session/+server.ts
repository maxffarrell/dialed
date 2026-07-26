import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = ({ cookies }) => json({
  authenticated: Boolean(cookies.get('dialed_session')),
  user: cookies.get('dialed_session') ? { id: 'demo-user', name: 'You' } : null
});
