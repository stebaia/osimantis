import 'package:dio/dio.dart';
import 'package:get_it/get_it.dart';

import '../../features/chat/data/datasources/chat_remote_datasource.dart';
import '../../features/chat/data/repositories/chat_repository_impl.dart';
import '../../features/chat/domain/repositories/chat_repository.dart';
import '../../features/chat/domain/usecases/send_message.dart';
import '../../features/chat/presentation/bloc/chat_bloc.dart';
import '../../features/graph/data/datasources/graph_remote_datasource.dart';
import '../../features/graph/data/repositories/graph_repository_impl.dart';
import '../../features/graph/domain/repositories/graph_repository.dart';
import '../../features/graph/domain/usecases/get_graph.dart';
import '../../features/person/data/datasources/person_remote_datasource.dart';
import '../../features/person/data/repositories/person_repository_impl.dart';
import '../../features/person/domain/repositories/person_repository.dart';
import '../../features/person/domain/usecases/get_person.dart';
import '../network/dio_client.dart';
import '../speech/speech_service.dart';

/// Service locator globale.
final sl = GetIt.instance;

/// Registra tutte le dipendenze. Da chiamare una volta in main(), prima di
/// runApp. Convenzione: singleton per servizi/repository, factory per i bloc
/// (uno nuovo per ogni schermata).
Future<void> configureDependencies() async {
  // Core
  sl.registerLazySingleton<Dio>(createDio);
  sl.registerLazySingleton<SpeechService>(SpeechService.new);

  // Chat — data
  sl.registerLazySingleton(() => ChatRemoteDataSource(sl()));
  sl.registerLazySingleton<ChatRepository>(() => ChatRepositoryImpl(sl()));

  // Chat — domain
  sl.registerLazySingleton(() => SendMessage(sl()));

  // Person — data
  sl.registerLazySingleton(() => PersonRemoteDataSource(sl()));
  sl.registerLazySingleton<PersonRepository>(() => PersonRepositoryImpl(sl()));

  // Person — domain
  sl.registerLazySingleton(() => GetPerson(sl()));

  // Graph — data
  sl.registerLazySingleton(() => GraphRemoteDataSource(sl()));
  sl.registerLazySingleton<GraphRepository>(() => GraphRepositoryImpl(sl()));

  // Graph — domain
  sl.registerLazySingleton(() => GetGraph(sl()));

  // Presentation
  sl.registerFactory(() => ChatBloc(sendMessage: sl(), getPerson: sl(), speech: sl()));
}
