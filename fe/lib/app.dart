import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'core/theme/app_theme.dart';
import 'features/chat/presentation/pages/chat_page.dart';

/// Root dell'app: tema, localizzazione (IT) e home chat-first.
class OsimantisApp extends StatelessWidget {
  const OsimantisApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Osimantis',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light,
      locale: const Locale('it'),
      supportedLocales: const [Locale('it'), Locale('en')],
      localizationsDelegates: const [
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      home: const ChatPage(),
    );
  }
}
