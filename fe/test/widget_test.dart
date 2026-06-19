import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:osimantis/core/theme/app_theme.dart';

void main() {
  testWidgets('il tema chiaro si applica senza errori', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.light,
        home: const Scaffold(body: Center(child: Text('Osimantis'))),
      ),
    );
    expect(find.text('Osimantis'), findsOneWidget);
  });
}
